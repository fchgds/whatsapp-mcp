// internal/wa/wa.go
package wa

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"whatsapp-mcp/internal/ingest"
	"whatsapp-mcp/internal/ipc"
)

type Client struct {
	wm     *whatsmeow.Client
	mu     sync.Mutex
	needQR bool
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
	c := &Client{wm: wm}
	wm.AddEventHandler(h.Handle)
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
	if c.wm.Store.ID != nil {
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

var _ = events.Message{} // mantiene el import si se referencia en evolución
