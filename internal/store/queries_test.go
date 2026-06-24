// internal/store/queries_test.go
package store

import (
	"testing"
	"time"

	"whatsapp-mcp/internal/model"
)

func seed(t *testing.T, s *Store) {
	t.Helper()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
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

func TestSyncContactNamesFillsEmptyChatName(t *testing.T) {
	s := openTemp(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.UpsertChat(model.Chat{JID: "5493333@s.whatsapp.net", Type: "individual", LastMessageText: "hola", LastMessageTS: time.Unix(1700000000, 0)}))

	contacts, chats, err := s.SyncContactNames(map[string]string{"5493333@s.whatsapp.net": "Contacto Nuevo"})
	if err != nil {
		t.Fatal(err)
	}
	if contacts != 1 || chats != 1 {
		t.Fatalf("contadores inesperados contacts=%d chats=%d", contacts, chats)
	}
	got, err := s.ListChats("Nuevo", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Contacto Nuevo" {
		t.Fatalf("chat con nombre sincronizado inesperado: %+v", got)
	}
}

func TestListChatsUsesContactNameFallback(t *testing.T) {
	s := openTemp(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.UpsertContact(model.Contact{JID: "5494444@s.whatsapp.net", Name: "Nombre Agenda", Phone: "5494444"}))
	must(s.UpsertChat(model.Chat{JID: "5494444@s.whatsapp.net", Type: "individual", LastMessageText: "hola", LastMessageTS: time.Unix(1700000000, 0)}))

	got, err := s.ResolveChats("Agenda")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Nombre Agenda" {
		t.Fatalf("esperaba fallback por contacto, got %+v", got)
	}
}

func TestUpsertChatDoesNotReplaceNewerLastMessageWithOlderSync(t *testing.T) {
	s := openTemp(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	jid := "5495555@s.whatsapp.net"
	must(s.UpsertChat(model.Chat{
		JID:             jid,
		Name:            "Chat",
		Type:            "individual",
		LastMessageText: "nuevo",
		LastMessageTS:   time.Unix(1700000100, 0),
	}))
	must(s.UpsertChat(model.Chat{
		JID:             jid,
		Name:            "Chat",
		Type:            "individual",
		LastMessageText: "viejo",
		LastMessageTS:   time.Unix(1700000000, 0),
	}))

	got, err := s.ListChats("Chat", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].LastMessageText != "nuevo" || !got[0].LastMessageTS.Equal(time.Unix(1700000100, 0)) {
		t.Fatalf("el último mensaje nuevo fue reemplazado: %+v", got)
	}
}
