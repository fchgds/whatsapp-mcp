// internal/wa/wa.go
package wa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waMmsRetry"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"whatsapp-mcp/internal/ingest"
	"whatsapp-mcp/internal/ipc"
)

type Client struct {
	wm           *whatsmeow.Client
	mu           sync.Mutex
	needQR       bool
	mediaRetryMu sync.Mutex
	mediaRetries map[types.MessageID]chan *events.MediaRetry
}

func New(ctx context.Context, sessionDBPath string, h *ingest.Handler) (*Client, error) {
	dbLog := waLog.Stdout("Database", "WARN", true)
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", sessionDBPath)
	container, err := sqlstore.New(ctx, "sqlite", dsn, dbLog)
	if err != nil {
		return nil, err
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, err
	}
	wm := whatsmeow.NewClient(device, waLog.Stdout("Client", "WARN", true))
	c := &Client{wm: wm, mediaRetries: make(map[types.MessageID]chan *events.MediaRetry)}
	wm.AddEventHandler(h.Handle)
	wm.AddEventHandler(c.Handle)
	return c, nil
}

// Start conecta. Si el device no tiene sesión (Store.ID == nil),
// abre el canal de QR y lo imprime en la terminal del daemon.
func (c *Client) Start(ctx context.Context) error {
	if c.wm.Store.ID == nil {
		qrChan, err := c.wm.GetQRChannel(ctx)
		if err != nil {
			return err
		}
		if err := c.wm.Connect(); err != nil {
			return err
		}
		go func() {
			for evt := range qrChan {
				switch evt.Event {
				case "code":
					c.setNeedQR(true)
					fmt.Println("\nEscaneá este QR desde WhatsApp → Dispositivos vinculados:")
					qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				case "success":
					c.setNeedQR(false)
					fmt.Println("Vinculado correctamente.")
				}
			}
		}()
		return nil
	}
	return c.wm.Connect()
}

func (c *Client) setNeedQR(v bool) { c.mu.Lock(); c.needQR = v; c.mu.Unlock() }

func (c *Client) Status() ipc.Status {
	c.mu.Lock()
	needQR := c.needQR
	c.mu.Unlock()
	st := ipc.Status{Connected: c.wm.IsConnected() && c.wm.IsLoggedIn(), NeedsQR: needQR}
	if c.wm.Store != nil && c.wm.Store.ID != nil {
		st.JID = c.wm.Store.ID.String()
	}
	return st
}

// DownloadAny deserializa el proto guardado y descarga su media.
func (c *Client) DownloadAny(ctx context.Context, rawProto []byte) ([]byte, error) {
	var msg waE2E.Message
	if err := proto.Unmarshal(rawProto, &msg); err != nil {
		return nil, err
	}
	return c.wm.DownloadAny(ctx, &msg)
}

func (c *Client) DownloadAnyWithMediaRetry(ctx context.Context, info types.MessageInfo, rawProto []byte) ([]byte, []byte, error) {
	var msg waE2E.Message
	if err := proto.Unmarshal(rawProto, &msg); err != nil {
		return nil, nil, err
	}
	data, err := c.wm.DownloadAny(ctx, &msg)
	if err == nil || !isExpiredMediaError(err) {
		return data, rawProto, err
	}
	media, ok := downloadableMedia(&msg)
	if !ok {
		return nil, rawProto, err
	}
	retryEvt, retryErr := c.requestMediaRetry(ctx, &info, media.GetMediaKey())
	if retryErr != nil {
		return nil, rawProto, fmt.Errorf("%w; media retry failed: %v", err, retryErr)
	}
	retryData, retryErr := whatsmeow.DecryptMediaRetryNotification(retryEvt, media.GetMediaKey())
	if retryErr != nil {
		return nil, rawProto, fmt.Errorf("%w; media retry decrypt failed: %v", err, retryErr)
	}
	if retryData.GetResult() != waMmsRetry.MediaRetryNotification_SUCCESS || retryData.GetDirectPath() == "" {
		return nil, rawProto, fmt.Errorf("%w; media retry result: %s", err, retryData.GetResult())
	}
	setDirectPath(&msg, retryData.GetDirectPath())
	data, retryErr = c.wm.DownloadAny(ctx, &msg)
	if retryErr != nil {
		return nil, rawProto, retryErr
	}
	updatedProto, retryErr := proto.Marshal(&msg)
	if retryErr != nil {
		return data, rawProto, nil
	}
	return data, updatedProto, nil
}

func (c *Client) Handle(evt any) {
	retryEvt, ok := evt.(*events.MediaRetry)
	if !ok {
		return
	}
	c.mediaRetryMu.Lock()
	ch := c.mediaRetries[retryEvt.MessageID]
	c.mediaRetryMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- retryEvt:
	default:
	}
}

func (c *Client) requestMediaRetry(ctx context.Context, info *types.MessageInfo, mediaKey []byte) (*events.MediaRetry, error) {
	ch := make(chan *events.MediaRetry, 1)
	c.mediaRetryMu.Lock()
	c.mediaRetries[info.ID] = ch
	c.mediaRetryMu.Unlock()
	defer func() {
		c.mediaRetryMu.Lock()
		delete(c.mediaRetries, info.ID)
		c.mediaRetryMu.Unlock()
	}()
	if err := c.wm.SendMediaRetryReceipt(ctx, info, mediaKey); err != nil {
		return nil, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		select {
		case evt := <-ch:
			return evt, nil
		case <-waitCtx.Done():
			return nil, waitCtx.Err()
		}
	}
}

func isExpiredMediaError(err error) bool {
	return errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith404) ||
		errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith410)
}

func downloadableMedia(msg *waE2E.Message) (whatsmeow.DownloadableMessage, bool) {
	switch {
	case msg.GetImageMessage() != nil:
		return msg.GetImageMessage(), true
	case msg.GetVideoMessage() != nil:
		return msg.GetVideoMessage(), true
	case msg.GetAudioMessage() != nil:
		return msg.GetAudioMessage(), true
	case msg.GetDocumentMessage() != nil:
		return msg.GetDocumentMessage(), true
	case msg.GetStickerMessage() != nil:
		return msg.GetStickerMessage(), true
	default:
		return nil, false
	}
}

func setDirectPath(msg *waE2E.Message, directPath string) {
	switch {
	case msg.GetImageMessage() != nil:
		msg.GetImageMessage().DirectPath = proto.String(directPath)
	case msg.GetVideoMessage() != nil:
		msg.GetVideoMessage().DirectPath = proto.String(directPath)
	case msg.GetAudioMessage() != nil:
		msg.GetAudioMessage().DirectPath = proto.String(directPath)
	case msg.GetDocumentMessage() != nil:
		msg.GetDocumentMessage().DirectPath = proto.String(directPath)
	case msg.GetStickerMessage() != nil:
		msg.GetStickerMessage().DirectPath = proto.String(directPath)
	}
}

// SyncContacts lee la store interna de whatsmeow y devuelve un mapa JID→nombre.
func (c *Client) SyncContacts(ctx context.Context) (map[string]string, error) {
	if c.wm.Store == nil || c.wm.Store.Contacts == nil {
		return nil, nil
	}
	all, err := c.wm.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(all))
	for jid, info := range all {
		name := info.FullName
		if name == "" {
			name = info.FirstName
		}
		if name == "" {
			name = info.PushName
		}
		if name == "" {
			name = info.BusinessName
		}
		if name != "" {
			out[jid.String()] = name
			if c.wm.Store.LIDs != nil && jid.Server == types.DefaultUserServer {
				lid, err := c.wm.Store.LIDs.GetLIDForPN(ctx, jid)
				if err == nil && !lid.IsEmpty() {
					out[lid.String()] = name
				}
			}
		}
	}
	return out, nil
}

var _ = events.Message{} // mantiene el import si se referencia en evolución
