// internal/store/queries_test.go
package store

import (
	"testing"
	"time"

	"whatsapp-mcp/internal/model"
)

func seed(t *testing.T, s *Store) {
	t.Helper()
	must := func(err error) { if err != nil { t.Fatal(err) } }
	must(s.UpsertContact(model.Contact{JID: "5491111@s.whatsapp.net", Name: "Fulano", Phone: "5491111"}))
	must(s.UpsertContact(model.Contact{JID: "5492222@s.whatsapp.net", Name: "Mengano", Phone: "5492222"}))
	must(s.UpsertChat(model.Chat{JID: "5491111@s.whatsapp.net", Name: "Fulano", Type: "individual"}))
	base := time.Unix(1700000000, 0)
	must(s.InsertMessage(model.Message{ID: "m1", ChatJID: "5491111@s.whatsapp.net", SenderJID: "5491111@s.whatsapp.net", Timestamp: base, Type: "text", Text: "hola"}))
	must(s.InsertMessage(model.Message{ID: "m2", ChatJID: "5491111@s.whatsapp.net", SenderJID: "5491111@s.whatsapp.net", Timestamp: base.Add(time.Minute), Type: "image", Media: &model.MediaInfo{Mimetype: "image/jpeg", Size: 1234}, RawProto: []byte("proto")}))
}

func TestSearchContacts(t *testing.T) {
	s := openTemp(t)
	seed(t, s)
	got, err := s.SearchContacts("ful", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Fulano" {
		t.Fatalf("esperaba [Fulano], got %+v", got)
	}
}

func TestGetMessagesChronological(t *testing.T) {
	s := openTemp(t)
	seed(t, s)
	msgs, err := s.GetMessages("5491111@s.whatsapp.net", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].ID != "m1" || msgs[1].ID != "m2" {
		t.Fatalf("orden/contenido inesperado: %+v", msgs)
	}
}

func TestListMediaOnlyMedia(t *testing.T) {
	s := openTemp(t)
	seed(t, s)
	media, err := s.ListMedia("5491111@s.whatsapp.net", nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(media) != 1 || media[0].ID != "m2" || string(media[0].RawProto) != "proto" {
		t.Fatalf("esperaba sólo m2 con raw_proto, got %+v", media)
	}
}

func TestResolveChatsByJID(t *testing.T) {
	s := openTemp(t)
	seed(t, s)
	chats, err := s.ResolveChats("5491111@s.whatsapp.net")
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 {
		t.Fatalf("esperaba 1 chat exacto, got %d", len(chats))
	}
}
