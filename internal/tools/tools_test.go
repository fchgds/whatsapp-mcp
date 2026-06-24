package tools

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"whatsapp-mcp/internal/ipc"
	"whatsapp-mcp/internal/model"
	"whatsapp-mcp/internal/store"
)

type fakeDaemon struct {
	status ipc.Status
}

func (f fakeDaemon) Status(context.Context) (ipc.Status, error) {
	return f.status, nil
}

func (f fakeDaemon) Download(context.Context, ipc.DownloadRequest) (ipc.DownloadResult, error) {
	return ipc.DownloadResult{}, nil
}

func seededTools(t *testing.T) *Tools {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	_ = s.UpsertContact(model.Contact{JID: "5491111@s.whatsapp.net", Name: "Fulano", Phone: "5491111"})
	_ = s.UpsertChat(model.Chat{JID: "5491111@s.whatsapp.net", Name: "Fulano", Type: "individual"})
	_ = s.InsertMessage(model.Message{ID: "m1", ChatJID: "5491111@s.whatsapp.net", SenderJID: "5491111@s.whatsapp.net", Timestamp: time.Unix(1700000000, 0), Type: "text", Text: "hola"})
	return &Tools{Store: s, Daemon: fakeDaemon{status: ipc.Status{Connected: true, JID: "5491111@s.whatsapp.net"}}}
}

func TestSearchContactsHandler(t *testing.T) {
	tl := seededTools(t)
	_, out, err := tl.SearchContacts(context.Background(), nil, SearchContactsIn{Query: "ful"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Contacts) != 1 || out.Contacts[0].Name != "Fulano" {
		t.Fatalf("esperaba Fulano, got %+v", out.Contacts)
	}
}

func TestGetMessagesResolvesByName(t *testing.T) {
	tl := seededTools(t)
	_, out, err := tl.GetMessages(context.Background(), nil, GetMessagesIn{Chat: "Fulano", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Messages) != 1 || out.Messages[0].Text != "hola" {
		t.Fatalf("mensajes inesperados: %+v", out.Messages)
	}
}

func TestGetMessagesAmbiguousReturnsCandidates(t *testing.T) {
	tl := seededTools(t)
	_ = tl.Store.UpsertChat(model.Chat{JID: "5493333@s.whatsapp.net", Name: "Fulano de Tal", Type: "individual"})
	_, out, err := tl.GetMessages(context.Background(), nil, GetMessagesIn{Chat: "Fulano", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Candidates) < 2 {
		t.Fatalf("esperaba candidatos por ambigüedad, got %+v", out)
	}
}
