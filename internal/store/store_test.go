package store

import (
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenCreatesSchema(t *testing.T) {
	s := openTemp(t)
	var n int
	row := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('chats','contacts','messages')`)
	if err := row.Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("esperaba 3 tablas, hay %d", n)
	}
}
