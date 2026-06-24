// internal/wa/backend.go
package wa

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"go.mau.fi/whatsmeow/types"

	"whatsapp-mcp/internal/ipc"
	"whatsapp-mcp/internal/store"
)

type Backend struct {
	Store *store.Store
	WA    *Client
}

func (b *Backend) Status() ipc.Status { return b.WA.Status() }

func (b *Backend) SyncContacts(ctx context.Context) (ipc.SyncContactsResult, error) {
	names, err := b.WA.SyncContacts(ctx)
	if err != nil {
		return ipc.SyncContactsResult{}, err
	}
	contacts, chats, err := b.Store.SyncContactNames(names)
	if err != nil {
		return ipc.SyncContactsResult{}, err
	}
	return ipc.SyncContactsResult{Contacts: contacts, Chats: chats}, nil
}

func (b *Backend) Download(ctx context.Context, req ipc.DownloadRequest) (ipc.DownloadResult, error) {
	chats, err := b.Store.ResolveChats(req.Chat)
	if err != nil {
		return ipc.DownloadResult{}, err
	}
	if len(chats) == 0 {
		return ipc.DownloadResult{}, fmt.Errorf("no encontré el chat %q", req.Chat)
	}
	if len(chats) > 1 {
		return ipc.DownloadResult{}, fmt.Errorf("%q es ambiguo (%d coincidencias); usá el JID exacto", req.Chat, len(chats))
	}
	limit := req.Limit
	if limit == 0 {
		limit = 50
	}
	media, err := b.Store.ListMedia(chats[0].JID, req.Types, limit)
	if err != nil {
		return ipc.DownloadResult{}, err
	}
	if err := os.MkdirAll(req.DestFolder, 0o755); err != nil {
		return ipc.DownloadResult{}, err
	}
	var res ipc.DownloadResult
	for _, m := range media {
		info, err := b.messageInfo(m.ID, m.ChatJID, m.SenderJID)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", m.ID, err))
			continue
		}
		data, rawProto, err := b.WA.DownloadAnyWithMediaRetry(ctx, info, m.RawProto)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", m.ID, err))
			continue
		}
		if !bytes.Equal(rawProto, m.RawProto) {
			if err := b.Store.UpdateMessageRawProto(m.ChatJID, m.ID, rawProto); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: downloaded but failed to persist refreshed media reference: %v", m.ID, err))
			}
		}
		name := fileName(m.ID, m.Media.Filename, m.Media.Mimetype)
		path := filepath.Join(req.DestFolder, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", m.ID, err))
			continue
		}
		res.Files = append(res.Files, ipc.SavedFile{Path: path, Mimetype: m.Media.Mimetype, Size: int64(len(data))})
	}
	return res, nil
}

func (b *Backend) messageInfo(id, chatJID, senderJID string) (types.MessageInfo, error) {
	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return types.MessageInfo{}, fmt.Errorf("chat JID inválido: %w", err)
	}
	sender, err := types.ParseJID(senderJID)
	if err != nil {
		return types.MessageInfo{}, fmt.Errorf("sender JID inválido: %w", err)
	}
	own := b.WA.wm.Store.ID.ToNonAD()
	isFromMe := !own.IsEmpty() && sender.ToNonAD() == own
	return types.MessageInfo{
		ID: types.MessageID(id),
		MessageSource: types.MessageSource{
			Chat:     chat,
			Sender:   sender,
			IsFromMe: isFromMe,
			IsGroup:  chat.Server == types.GroupServer,
		},
	}, nil
}

func fileName(id, original, mimetype string) string {
	if original != "" {
		return filepath.Base(original)
	}
	ext := ".bin"
	if exts, _ := mime.ExtensionsByType(mimetype); len(exts) > 0 {
		ext = exts[0]
	}
	return id + ext
}

var _ ipc.Backend = (*Backend)(nil)
