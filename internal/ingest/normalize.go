// internal/ingest/normalize.go
package ingest

import (
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"whatsapp-mcp/internal/model"
)

// NormalizeMessage convierte un *events.Message en model.Message.
// Devuelve ok=false si el evento no tiene contenido que nos interese.
func NormalizeMessage(evt *events.Message) (model.Message, bool) {
	if evt == nil || evt.Message == nil {
		return model.Message{}, false
	}
	m := model.Message{
		ID:        evt.Info.ID,
		ChatJID:   evt.Info.Chat.String(),
		SenderJID: evt.Info.Sender.String(),
		Timestamp: evt.Info.Timestamp,
	}
	msg := evt.Message

	switch {
	case msg.GetConversation() != "":
		m.Type, m.Text = "text", msg.GetConversation()
	case msg.GetExtendedTextMessage() != nil:
		m.Type, m.Text = "text", msg.GetExtendedTextMessage().GetText()
	case msg.GetImageMessage() != nil:
		setMedia(&m, "image", msg.GetImageMessage().GetMimetype(), "", msg.GetImageMessage().GetFileLength())
		m.Text = msg.GetImageMessage().GetCaption()
	case msg.GetVideoMessage() != nil:
		setMedia(&m, "video", msg.GetVideoMessage().GetMimetype(), "", msg.GetVideoMessage().GetFileLength())
		m.Text = msg.GetVideoMessage().GetCaption()
	case msg.GetAudioMessage() != nil:
		setMedia(&m, "audio", msg.GetAudioMessage().GetMimetype(), "", msg.GetAudioMessage().GetFileLength())
	case msg.GetDocumentMessage() != nil:
		d := msg.GetDocumentMessage()
		setMedia(&m, "document", d.GetMimetype(), d.GetFileName(), d.GetFileLength())
		m.Text = d.GetCaption()
	case msg.GetStickerMessage() != nil:
		setMedia(&m, "sticker", msg.GetStickerMessage().GetMimetype(), "", msg.GetStickerMessage().GetFileLength())
	default:
		return model.Message{}, false
	}

	if m.Media != nil {
		raw, err := proto.Marshal(msg)
		if err == nil {
			m.RawProto = raw
		}
	}
	return m, true
}

func setMedia(m *model.Message, typ, mime, filename string, size uint64) {
	m.Type = typ
	m.Media = &model.MediaInfo{Mimetype: mime, Filename: filename, Size: int64(size)}
}
