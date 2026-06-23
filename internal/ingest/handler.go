// internal/ingest/handler.go
package ingest

import (
	"time"

	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"whatsapp-mcp/internal/model"
	"whatsapp-mcp/internal/store"
)

type Handler struct {
	Store *store.Store
}

func (h *Handler) Handle(evt any) {
	switch v := evt.(type) {
	case *events.Message:
		h.onMessage(v)
	case *events.HistorySync:
		for _, conv := range v.Data.GetConversations() {
			for _, hm := range conv.GetMessages() {
				wmsg := hm.GetMessage()
				if wmsg == nil {
					continue
				}
				h.onMessage(&events.Message{Info: parseHistoryInfo(wmsg), Message: wmsg.GetMessage()})
			}
		}
	}
}

func (h *Handler) onMessage(evt *events.Message) {
	m, ok := NormalizeMessage(evt)
	if !ok {
		return
	}
	_ = h.Store.InsertMessage(m)
	_ = h.Store.UpsertChat(model.Chat{
		JID:             m.ChatJID,
		Type:            chatType(m.ChatJID),
		LastMessageText: previewText(m),
		LastMessageTS:   m.Timestamp,
	})
}

func parseHistoryInfo(wm *waWeb.WebMessageInfo) types.MessageInfo {
	remoteJID := wm.GetKey().GetRemoteJID()
	chat, _ := types.ParseJID(remoteJID)
	senderStr := remoteJID
	if p := wm.GetKey().GetParticipant(); p != "" {
		senderStr = p
	}
	sender, _ := types.ParseJID(senderStr)
	return types.MessageInfo{
		ID:        wm.GetKey().GetID(),
		Timestamp: time.Unix(int64(wm.GetMessageTimestamp()), 0),
		MessageSource: types.MessageSource{
			Chat:   chat,
			Sender: sender,
		},
	}
}

func chatType(jid string) string {
	if len(jid) > 5 && jid[len(jid)-5:] == "@g.us" {
		return "group"
	}
	return "individual"
}

func previewText(m model.Message) string {
	if m.Text != "" {
		return m.Text
	}
	if m.Media != nil {
		return "[" + m.Type + "]"
	}
	return ""
}
