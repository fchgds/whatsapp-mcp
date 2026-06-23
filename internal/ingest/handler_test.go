// internal/ingest/handler_test.go
package ingest

import (
	"path/filepath"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"whatsapp-mcp/internal/store"
)

func TestHandleMessagePersists(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := &Handler{Store: s}

	h.Handle(&events.Message{
		Info:    mkInfo("5491111", "5491111", "m1"),
		Message: &waE2E.Message{Conversation: proto.String("hola")},
	})

	msgs, err := s.GetMessages(events.Message{Info: mkInfo("5491111", "5491111", "m1")}.Info.Chat.String(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Text != "hola" {
		t.Fatalf("el mensaje no se persistió: %+v", msgs)
	}
}
