package ipc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeBackend struct{}

func (fakeBackend) Status() Status { return Status{Connected: true, JID: "me@s.whatsapp.net"} }
func (fakeBackend) Download(_ context.Context, r DownloadRequest) (DownloadResult, error) {
	return DownloadResult{Files: []SavedFile{{Path: r.DestFolder + "/a.jpg", Mimetype: "image/jpeg", Size: 10}}}, nil
}

func TestStatusRequiresToken(t *testing.T) {
	srv := httptest.NewServer(NewServer("secret", fakeBackend{}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Token: "wrong", HTTP: srv.Client()}
	if _, err := c.Status(context.Background()); err == nil {
		t.Fatal("token inválido debería fallar")
	}
}

func TestStatusAndDownloadRoundTrip(t *testing.T) {
	srv := httptest.NewServer(NewServer("secret", fakeBackend{}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Token: "secret", HTTP: srv.Client()}

	st, err := c.Status(context.Background())
	if err != nil || !st.Connected || st.JID != "me@s.whatsapp.net" {
		t.Fatalf("status inesperado: %+v err=%v", st, err)
	}
	res, err := c.Download(context.Background(), DownloadRequest{Chat: "x", DestFolder: "/tmp"})
	if err != nil || len(res.Files) != 1 || res.Files[0].Path != "/tmp/a.jpg" {
		t.Fatalf("download inesperado: %+v err=%v", res, err)
	}
}

var _ http.Handler = NewServer("t", fakeBackend{})
