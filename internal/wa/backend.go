// internal/wa/backend.go
package wa

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"whatsapp-mcp/internal/ipc"
	"whatsapp-mcp/internal/store"
)

type Backend struct {
	Store *store.Store
	WA    *Client
}

func (b *Backend) Status() ipc.Status { return b.WA.Status() }

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
		data, err := b.WA.DownloadAny(ctx, m.RawProto)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", m.ID, err))
			continue
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
