package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"whatsapp-mcp/internal/ipc"
	"whatsapp-mcp/internal/model"
	"whatsapp-mcp/internal/store"
)

type DaemonClient interface {
	Status(ctx context.Context) (ipc.Status, error)
	Download(ctx context.Context, req ipc.DownloadRequest) (ipc.DownloadResult, error)
}

type Tools struct {
	Store  *store.Store
	Daemon DaemonClient
}

// ---- DTOs de salida ----

type ContactDTO struct {
	JID   string `json:"jid"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type ChatDTO struct {
	JID         string `json:"jid"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	LastMessage string `json:"last_message"`
}

type MessageDTO struct {
	ID        string `json:"id"`
	SenderJID string `json:"sender_jid"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      string `json:"text"`
	HasMedia  bool   `json:"has_media"`
	Mimetype  string `json:"mimetype,omitempty"`
}

func toChatDTO(c model.Chat) ChatDTO {
	return ChatDTO{JID: c.JID, Name: c.Name, Type: c.Type, LastMessage: c.LastMessageText}
}

func toMessageDTO(m model.Message) MessageDTO {
	d := MessageDTO{ID: m.ID, SenderJID: m.SenderJID, Timestamp: m.Timestamp.Format("2006-01-02 15:04"), Type: m.Type, Text: m.Text}
	if m.Media != nil {
		d.HasMedia = true
		d.Mimetype = m.Media.Mimetype
	}
	return d
}

// ---- get_connection_status ----

type StatusIn struct{}
type StatusOut struct {
	Connected bool   `json:"connected"`
	NeedsQR   bool   `json:"needs_qr"`
	JID       string `json:"jid"`
}

func (t *Tools) GetConnectionStatus(ctx context.Context, _ *mcp.CallToolRequest, _ StatusIn) (*mcp.CallToolResult, StatusOut, error) {
	if t.Daemon == nil {
		return nil, StatusOut{}, fmt.Errorf("daemon no disponible: arrancá whatsapp-daemon.exe")
	}
	st, err := t.Daemon.Status(ctx)
	if err != nil {
		return nil, StatusOut{}, fmt.Errorf("daemon no disponible: %w", err)
	}
	return nil, StatusOut{Connected: st.Connected, NeedsQR: st.NeedsQR, JID: st.JID}, nil
}

// ---- search_contacts ----

type SearchContactsIn struct {
	Query string `json:"query" jsonschema:"texto a buscar en nombre o teléfono"`
	Limit int    `json:"limit,omitempty" jsonschema:"máximo de resultados (default 20)"`
}
type SearchContactsOut struct {
	Contacts []ContactDTO `json:"contacts"`
}

func (t *Tools) SearchContacts(ctx context.Context, _ *mcp.CallToolRequest, in SearchContactsIn) (*mcp.CallToolResult, SearchContactsOut, error) {
	cs, err := t.Store.SearchContacts(in.Query, limitOr(in.Limit, 20))
	if err != nil {
		return nil, SearchContactsOut{}, err
	}
	out := SearchContactsOut{}
	for _, c := range cs {
		out.Contacts = append(out.Contacts, ContactDTO{JID: c.JID, Name: c.Name, Phone: c.Phone})
	}
	return nil, out, nil
}

// ---- list_chats ----

type ListChatsIn struct {
	Query string `json:"query,omitempty" jsonschema:"filtro opcional por nombre"`
	Limit int    `json:"limit,omitempty" jsonschema:"máximo de chats (default 20)"`
}
type ListChatsOut struct {
	Chats []ChatDTO `json:"chats"`
}

func (t *Tools) ListChats(ctx context.Context, _ *mcp.CallToolRequest, in ListChatsIn) (*mcp.CallToolResult, ListChatsOut, error) {
	cs, err := t.Store.ListChats(in.Query, limitOr(in.Limit, 20))
	if err != nil {
		return nil, ListChatsOut{}, err
	}
	out := ListChatsOut{}
	for _, c := range cs {
		out.Chats = append(out.Chats, toChatDTO(c))
	}
	return nil, out, nil
}

// ---- get_messages ----

type GetMessagesIn struct {
	Chat  string `json:"chat" jsonschema:"nombre o JID del chat"`
	Limit int    `json:"limit,omitempty" jsonschema:"máximo de mensajes (default 50)"`
}
type GetMessagesOut struct {
	ChatJID    string       `json:"chat_jid,omitempty"`
	Messages   []MessageDTO `json:"messages,omitempty"`
	Candidates []ChatDTO    `json:"candidates,omitempty"`
}

func (t *Tools) GetMessages(ctx context.Context, _ *mcp.CallToolRequest, in GetMessagesIn) (*mcp.CallToolResult, GetMessagesOut, error) {
	jid, cands, err := t.resolveChat(in.Chat)
	if err != nil {
		return nil, GetMessagesOut{}, err
	}
	if jid == "" {
		return nil, GetMessagesOut{Candidates: cands}, nil
	}
	msgs, err := t.Store.GetMessages(jid, limitOr(in.Limit, 50))
	if err != nil {
		return nil, GetMessagesOut{}, err
	}
	out := GetMessagesOut{ChatJID: jid}
	for _, m := range msgs {
		out.Messages = append(out.Messages, toMessageDTO(m))
	}
	return nil, out, nil
}

// ---- list_media ----

type ListMediaIn struct {
	Chat  string   `json:"chat" jsonschema:"nombre o JID del chat"`
	Types []string `json:"types,omitempty" jsonschema:"filtrar por tipos: image,video,audio,document,sticker"`
	Limit int      `json:"limit,omitempty" jsonschema:"máximo (default 50)"`
}
type ListMediaOut struct {
	ChatJID    string       `json:"chat_jid,omitempty"`
	Media      []MessageDTO `json:"media,omitempty"`
	Candidates []ChatDTO    `json:"candidates,omitempty"`
}

func (t *Tools) ListMedia(ctx context.Context, _ *mcp.CallToolRequest, in ListMediaIn) (*mcp.CallToolResult, ListMediaOut, error) {
	jid, cands, err := t.resolveChat(in.Chat)
	if err != nil {
		return nil, ListMediaOut{}, err
	}
	if jid == "" {
		return nil, ListMediaOut{Candidates: cands}, nil
	}
	media, err := t.Store.ListMedia(jid, in.Types, limitOr(in.Limit, 50))
	if err != nil {
		return nil, ListMediaOut{}, err
	}
	out := ListMediaOut{ChatJID: jid}
	for _, m := range media {
		out.Media = append(out.Media, toMessageDTO(m))
	}
	return nil, out, nil
}

// ---- download_media ----

type DownloadMediaIn struct {
	Chat       string   `json:"chat" jsonschema:"nombre o JID del chat"`
	DestFolder string   `json:"dest_folder" jsonschema:"carpeta destino absoluta donde guardar los archivos"`
	Types      []string `json:"types,omitempty" jsonschema:"filtrar por tipos: image,video,audio,document,sticker"`
	Limit      int      `json:"limit,omitempty" jsonschema:"máximo de archivos (default 50)"`
}
type DownloadMediaOut struct {
	Files  []ipc.SavedFile `json:"files"`
	Errors []string        `json:"errors,omitempty"`
}

func (t *Tools) DownloadMedia(ctx context.Context, _ *mcp.CallToolRequest, in DownloadMediaIn) (*mcp.CallToolResult, DownloadMediaOut, error) {
	if t.Daemon == nil {
		return nil, DownloadMediaOut{}, fmt.Errorf("daemon no disponible: arrancá whatsapp-daemon.exe")
	}
	if in.DestFolder == "" {
		return nil, DownloadMediaOut{}, fmt.Errorf("dest_folder es obligatorio")
	}
	res, err := t.Daemon.Download(ctx, ipc.DownloadRequest{
		Chat: in.Chat, DestFolder: in.DestFolder, Types: in.Types, Limit: in.Limit,
	})
	if err != nil {
		return nil, DownloadMediaOut{}, err
	}
	return nil, DownloadMediaOut{Files: res.Files, Errors: res.Errors}, nil
}

// ---- helpers ----

func limitOr(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// resolveChat devuelve (jid, nil) si hay match único; ("", candidatos) si es ambiguo.
func (t *Tools) resolveChat(chat string) (string, []ChatDTO, error) {
	chats, err := t.Store.ResolveChats(chat)
	if err != nil {
		return "", nil, err
	}
	if len(chats) == 0 {
		return "", nil, fmt.Errorf("no encontré ningún chat que coincida con %q", chat)
	}
	if len(chats) == 1 {
		return chats[0].JID, nil, nil
	}
	var cands []ChatDTO
	for _, c := range chats {
		cands = append(cands, toChatDTO(c))
	}
	return "", cands, nil
}
