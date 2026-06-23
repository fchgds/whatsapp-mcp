// internal/ingest/normalize_test.go
package ingest

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func mkInfo(chat, sender, id string) types.MessageInfo {
	return types.MessageInfo{
		ID:        id,
		Timestamp: time.Unix(1700000000, 0),
		MessageSource: types.MessageSource{
			Chat:   types.NewJID(chat, types.DefaultUserServer),
			Sender: types.NewJID(sender, types.DefaultUserServer),
		},
	}
}

func TestNormalizeTextMessage(t *testing.T) {
	evt := &events.Message{
		Info:    mkInfo("5491111", "5491111", "m1"),
		Message: &waE2E.Message{Conversation: proto.String("hola")},
	}
	m, ok := NormalizeMessage(evt)
	if !ok || m.Type != "text" || m.Text != "hola" || m.ID != "m1" {
		t.Fatalf("normalización de texto inesperada: %+v ok=%v", m, ok)
	}
	if m.RawProto != nil {
		t.Fatal("un texto no debería guardar raw_proto")
	}
}

func TestNormalizeImageMessage(t *testing.T) {
	evt := &events.Message{
		Info: mkInfo("5491111", "5491111", "m2"),
		Message: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			Mimetype:   proto.String("image/jpeg"),
			Caption:    proto.String("foto"),
			FileLength: proto.Uint64(1234),
		}},
	}
	m, ok := NormalizeMessage(evt)
	if !ok || m.Type != "image" || m.Media == nil || m.Media.Mimetype != "image/jpeg" {
		t.Fatalf("normalización de imagen inesperada: %+v ok=%v", m, ok)
	}
	if m.Text != "foto" {
		t.Fatalf("caption debería ir a Text, got %q", m.Text)
	}
	if len(m.RawProto) == 0 {
		t.Fatal("una imagen debería guardar raw_proto para descargar luego")
	}
}
