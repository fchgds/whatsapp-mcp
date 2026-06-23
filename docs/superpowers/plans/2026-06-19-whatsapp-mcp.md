# whatsapp-mcp Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Un servidor MCP en Go que se conecta a WhatsApp "como WhatsApp Web" (vía QR) y permite a Claude leer mensajes y descargar adjuntos.

**Architecture:** Dos binarios Go que comparten SQLite local. Un **daemon** siempre-activo es el único dueño de la conexión whatsmeow: ingiere mensajes a `messages.db` y expone una API HTTP local (token) para acciones vivas (estado, QR, descarga). Un **servidor MCP** por stdio responde a Claude: las lecturas las hace contra SQLite y las acciones vivas las delega al daemon.

**Tech Stack:** Go 1.26 · `go.mau.fi/whatsmeow` · `github.com/modelcontextprotocol/go-sdk` v1.6.1 · `modernc.org/sqlite` v1.52.0 (puro Go, sin CGO) · `github.com/mdp/qrterminal/v3` v3.2.1

## Global Constraints

- Go floor: **1.26.x** (whatsmeow exige ≥1.25). Verificar con `go version`.
- SQLite driver: **`modernc.org/sqlite`** (driver name `"sqlite"`). Import en blanco `_ "modernc.org/sqlite"`. **No** usar `mattn/go-sqlite3` (requiere CGO/gcc).
- Module path: `whatsapp-mcp`.
- whatsmeow import paths verificados (2026-06-19): logger `waLog "go.mau.fi/whatsmeow/util/log"`; proto `go.mau.fi/whatsmeow/proto/waE2E` + `google.golang.org/protobuf/proto`; eventos `go.mau.fi/whatsmeow/types/events`; tipos `go.mau.fi/whatsmeow/types`; sesión `go.mau.fi/whatsmeow/store/sqlstore` y `go.mau.fi/whatsmeow/store`.
- whatsmeow API verificada: `sqlstore.New(ctx, "sqlite", dsn, log) (*Container, error)` · `container.GetFirstDevice(ctx) (*store.Device, error)` · `whatsmeow.NewClient(device, log) *Client` · `client.AddEventHandler(func(evt any)) uint32` · `client.GetQRChannel(ctx) (<-chan QRChannelItem, error)` (item tiene `.Event`/`.Code`) · `client.DownloadAny(ctx, *waE2E.Message) ([]byte, error)`.
- Alcance: **solo lectura + descargas**. Sin envío.
- IPC siempre en `127.0.0.1`, protegida con token compartido.
- TDD: cada tarea testeable termina con sus tests en verde y un commit. Las tareas marcadas **[verificación manual]** integran whatsmeow real y no se testean en CI; se validan a mano según los pasos indicados.
- Datos sensibles: `data/` (session.db, messages.db) está gitignored.

---

### Task 1: Scaffold del módulo + config

**Files:**
- Create: `go.mod` (vía comando)
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `type Config struct { DataDir string; IPCPort int; IPCToken string }`
  - `func Load(path string) (Config, error)` — lee JSON; si el archivo no existe, devuelve defaults (`DataDir:"data"`, `IPCPort:8377`, token vacío→error claro).
  - `func (c Config) SessionDBPath() string` → `filepath.Join(DataDir,"session.db")`
  - `func (c Config) MessagesDBPath() string` → `filepath.Join(DataDir,"messages.db")`
  - `func (c Config) BaseURL() string` → `fmt.Sprintf("http://127.0.0.1:%d", IPCPort)`

- [ ] **Step 1: Inicializar el módulo y traer dependencias**

```bash
cd /c/dev/mcp/whatsapp-mcp
go mod init whatsapp-mcp
go get go.mau.fi/whatsmeow@latest
go get github.com/modelcontextprotocol/go-sdk@v1.6.1
go get modernc.org/sqlite@v1.52.0
go get github.com/mdp/qrterminal/v3@v3.2.1
go get google.golang.org/protobuf/proto
```
Expected: `go.mod` y `go.sum` creados; sin errores.

- [ ] **Step 2: Escribir el test que falla**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("se esperaba error porque falta el token")
	}
	_ = cfg
}

func TestLoadReadsValues(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	os.WriteFile(p, []byte(`{"data_dir":"d","ipc_port":9000,"ipc_token":"secret"}`), 0o600)

	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IPCPort != 9000 || cfg.IPCToken != "secret" || cfg.DataDir != "d" {
		t.Fatalf("config inesperada: %+v", cfg)
	}
	if cfg.MessagesDBPath() != filepath.Join("d", "messages.db") {
		t.Fatalf("MessagesDBPath mal: %s", cfg.MessagesDBPath())
	}
	if cfg.BaseURL() != "http://127.0.0.1:9000" {
		t.Fatalf("BaseURL mal: %s", cfg.BaseURL())
	}
}
```

- [ ] **Step 3: Correr el test y verlo fallar**

Run: `go test ./internal/config/`
Expected: FAIL (no compila: `Load` indefinido).

- [ ] **Step 4: Implementación mínima**

```go
// internal/config/config.go
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DataDir  string `json:"data_dir"`
	IPCPort  int    `json:"ipc_port"`
	IPCToken string `json:"ipc_token"`
}

func Load(path string) (Config, error) {
	cfg := Config{DataDir: "data", IPCPort: 8377}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("config inválida: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}
	if cfg.IPCToken == "" {
		return cfg, errors.New("falta ipc_token en config.json (generá uno aleatorio)")
	}
	return cfg, nil
}

func (c Config) SessionDBPath() string  { return filepath.Join(c.DataDir, "session.db") }
func (c Config) MessagesDBPath() string { return filepath.Join(c.DataDir, "messages.db") }
func (c Config) BaseURL() string        { return fmt.Sprintf("http://127.0.0.1:%d", c.IPCPort) }
```

- [ ] **Step 5: Correr tests (verde)**

Run: `go test ./internal/config/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config/
git commit -m "feat: scaffold del módulo y paquete config"
```

---

### Task 2: Modelo de dominio + apertura/schema del store

**Files:**
- Create: `internal/model/model.go`
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Produces (model):
  - `type Chat struct { JID, Name, Type, LastMessageText string; LastMessageTS time.Time }`
  - `type Contact struct { JID, Name, Phone string }`
  - `type MediaInfo struct { Mimetype, Filename string; Size int64 }`
  - `type Message struct { ID, ChatJID, SenderJID string; Timestamp time.Time; Type, Text string; Media *MediaInfo; RawProto []byte }`
- Produces (store):
  - `func Open(path string) (*Store, error)` — abre SQLite (modernc) y crea el schema.
  - `func (s *Store) Close() error`
  - `*Store` tiene un `db *sql.DB` no exportado.

- [ ] **Step 1: Escribir el test que falla**

```go
// internal/store/store_test.go
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
```

- [ ] **Step 2: Correr y verlo fallar**

Run: `go test ./internal/store/`
Expected: FAIL (`Open` indefinido).

- [ ] **Step 3: Implementar model**

```go
// internal/model/model.go
package model

import "time"

type Chat struct {
	JID             string
	Name            string
	Type            string // "individual" | "group"
	LastMessageText string
	LastMessageTS   time.Time
}

type Contact struct {
	JID   string
	Name  string
	Phone string
}

type MediaInfo struct {
	Mimetype string
	Filename string
	Size     int64
}

type Message struct {
	ID        string
	ChatJID   string
	SenderJID string
	Timestamp time.Time
	Type      string // text|image|audio|video|document|sticker
	Text      string
	Media     *MediaInfo // nil si no hay adjunto
	RawProto  []byte     // waE2E.Message serializado, sólo para mensajes con media
}
```

- [ ] **Step 4: Implementar store.Open + schema**

```go
// internal/store/store.go
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS chats (
  jid TEXT PRIMARY KEY,
  name TEXT,
  type TEXT,
  last_message_text TEXT,
  last_message_ts INTEGER
);
CREATE TABLE IF NOT EXISTS contacts (
  jid TEXT PRIMARY KEY,
  name TEXT,
  phone TEXT
);
CREATE TABLE IF NOT EXISTS messages (
  id TEXT,
  chat_jid TEXT,
  sender_jid TEXT,
  ts INTEGER,
  type TEXT,
  text TEXT,
  media_mimetype TEXT,
  media_filename TEXT,
  media_size INTEGER,
  raw_proto BLOB,
  PRIMARY KEY (id, chat_jid)
);
CREATE INDEX IF NOT EXISTS idx_messages_chat_ts ON messages(chat_jid, ts);
`

func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }
```

- [ ] **Step 5: Correr tests (verde)**

Run: `go test ./internal/store/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/model/ internal/store/
git commit -m "feat: modelo de dominio y apertura/schema del store SQLite"
```

---

### Task 3: API de escritura y lectura del store

**Files:**
- Create: `internal/store/queries.go`
- Test: `internal/store/queries_test.go`

**Interfaces:**
- Consumes: `*Store` (Task 2), `model.*` (Task 2)
- Produces (métodos de `*Store`):
  - `func (s *Store) UpsertChat(c model.Chat) error`
  - `func (s *Store) UpsertContact(c model.Contact) error`
  - `func (s *Store) InsertMessage(m model.Message) error`
  - `func (s *Store) SearchContacts(query string, limit int) ([]model.Contact, error)`
  - `func (s *Store) ListChats(query string, limit int) ([]model.Chat, error)`
  - `func (s *Store) ResolveChats(nameOrJID string) ([]model.Chat, error)` — exacto por JID o LIKE por nombre; base de la desambiguación.
  - `func (s *Store) GetMessages(chatJID string, limit int) ([]model.Message, error)` — orden cronológico ascendente, últimos `limit`.
  - `func (s *Store) ListMedia(chatJID string, types []string, limit int) ([]model.Message, error)` — sólo mensajes con media (incluye `RawProto`).

- [ ] **Step 1: Escribir el test que falla**

```go
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
```

- [ ] **Step 2: Correr y verlo fallar**

Run: `go test ./internal/store/ -run 'Search|GetMessages|ListMedia|Resolve'`
Expected: FAIL (métodos indefinidos).

- [ ] **Step 3: Implementar las queries**

```go
// internal/store/queries.go
package store

import (
	"strings"
	"time"

	"whatsapp-mcp/internal/model"
)

func (s *Store) UpsertChat(c model.Chat) error {
	_, err := s.db.Exec(`INSERT INTO chats (jid,name,type,last_message_text,last_message_ts)
		VALUES (?,?,?,?,?)
		ON CONFLICT(jid) DO UPDATE SET name=excluded.name, type=excluded.type,
			last_message_text=excluded.last_message_text, last_message_ts=excluded.last_message_ts`,
		c.JID, c.Name, c.Type, c.LastMessageText, c.LastMessageTS.Unix())
	return err
}

func (s *Store) UpsertContact(c model.Contact) error {
	_, err := s.db.Exec(`INSERT INTO contacts (jid,name,phone) VALUES (?,?,?)
		ON CONFLICT(jid) DO UPDATE SET name=excluded.name, phone=excluded.phone`,
		c.JID, c.Name, c.Phone)
	return err
}

func (s *Store) InsertMessage(m model.Message) error {
	var mime, fname string
	var size int64
	if m.Media != nil {
		mime, fname, size = m.Media.Mimetype, m.Media.Filename, m.Media.Size
	}
	_, err := s.db.Exec(`INSERT INTO messages
		(id,chat_jid,sender_jid,ts,type,text,media_mimetype,media_filename,media_size,raw_proto)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id,chat_jid) DO NOTHING`,
		m.ID, m.ChatJID, m.SenderJID, m.Timestamp.Unix(), m.Type, m.Text, mime, fname, size, m.RawProto)
	return err
}

func (s *Store) SearchContacts(query string, limit int) ([]model.Contact, error) {
	rows, err := s.db.Query(`SELECT jid,name,phone FROM contacts
		WHERE name LIKE ? OR phone LIKE ? ORDER BY name LIMIT ?`,
		"%"+query+"%", "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Contact
	for rows.Next() {
		var c model.Contact
		if err := rows.Scan(&c.JID, &c.Name, &c.Phone); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) ListChats(query string, limit int) ([]model.Chat, error) {
	rows, err := s.db.Query(`SELECT jid,name,type,last_message_text,last_message_ts FROM chats
		WHERE (?='' OR name LIKE ?) ORDER BY last_message_ts DESC LIMIT ?`,
		query, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChats(rows)
}

func (s *Store) ResolveChats(nameOrJID string) ([]model.Chat, error) {
	rows, err := s.db.Query(`SELECT jid,name,type,last_message_text,last_message_ts FROM chats
		WHERE jid=? OR name LIKE ? ORDER BY last_message_ts DESC LIMIT 20`,
		nameOrJID, "%"+nameOrJID+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChats(rows)
}

func (s *Store) GetMessages(chatJID string, limit int) ([]model.Message, error) {
	rows, err := s.db.Query(`SELECT id,chat_jid,sender_jid,ts,type,text,
		media_mimetype,media_filename,media_size,raw_proto
		FROM messages WHERE chat_jid=? ORDER BY ts DESC LIMIT ?`, chatJID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	// devolver en orden cronológico ascendente
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

func (s *Store) ListMedia(chatJID string, types []string, limit int) ([]model.Message, error) {
	q := `SELECT id,chat_jid,sender_jid,ts,type,text,
		media_mimetype,media_filename,media_size,raw_proto
		FROM messages WHERE chat_jid=? AND raw_proto IS NOT NULL`
	args := []any{chatJID}
	if len(types) > 0 {
		q += ` AND type IN (` + placeholders(len(types)) + `)`
		for _, t := range types {
			args = append(args, t)
		}
	}
	q += ` ORDER BY ts DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessages(rows)
}

func placeholders(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }

func scanChats(rows interface{ Next() bool; Scan(...any) error; Err() error }) ([]model.Chat, error) {
	var out []model.Chat
	for rows.Next() {
		var c model.Chat
		var ts int64
		if err := rows.Scan(&c.JID, &c.Name, &c.Type, &c.LastMessageText, &ts); err != nil {
			return nil, err
		}
		c.LastMessageTS = time.Unix(ts, 0)
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanMessages(rows interface{ Next() bool; Scan(...any) error; Err() error }) ([]model.Message, error) {
	var out []model.Message
	for rows.Next() {
		var m model.Message
		var ts, size int64
		var mime, fname string
		if err := rows.Scan(&m.ID, &m.ChatJID, &m.SenderJID, &ts, &m.Type, &m.Text,
			&mime, &fname, &size, &m.RawProto); err != nil {
			return nil, err
		}
		m.Timestamp = time.Unix(ts, 0)
		if m.RawProto != nil || mime != "" {
			m.Media = &model.MediaInfo{Mimetype: mime, Filename: fname, Size: size}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Correr tests (verde)**

Run: `go test ./internal/store/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: API de escritura/lectura del store (contactos, chats, mensajes, media)"
```

---

### Task 4: Normalización de eventos whatsmeow → model

**Files:**
- Create: `internal/ingest/normalize.go`
- Test: `internal/ingest/normalize_test.go`

**Interfaces:**
- Consumes: `model.*` (Task 2), tipos whatsmeow (`events`, `types`, `waE2E`).
- Produces:
  - `func NormalizeMessage(evt *events.Message) (model.Message, bool)` — convierte un evento a `model.Message`. El bool es `ok` (false si no hay contenido aprovechable). Para media, setea `Type`, `Media` y `RawProto = proto.Marshal(evt.Message)`.

- [ ] **Step 1: Escribir el test que falla**

```go
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
			Mimetype: proto.String("image/jpeg"),
			Caption:  proto.String("foto"),
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
```

- [ ] **Step 2: Correr y verlo fallar**

Run: `go test ./internal/ingest/`
Expected: FAIL (`NormalizeMessage` indefinido).

- [ ] **Step 3: Implementar normalize**

```go
// internal/ingest/normalize.go
package ingest

import (
	"go.mau.fi/whatsmeow/proto/waE2E"
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
```

- [ ] **Step 4: Correr tests (verde)**

Run: `go test ./internal/ingest/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ingest/normalize.go internal/ingest/normalize_test.go
git commit -m "feat: normalización de eventos whatsmeow a modelo de dominio"
```

---

### Task 5: Handler de ingesta (evento → store)

**Files:**
- Create: `internal/ingest/handler.go`
- Test: `internal/ingest/handler_test.go`

**Interfaces:**
- Consumes: `*store.Store` (Task 3), `NormalizeMessage` (Task 4), tipos `events`.
- Produces:
  - `type Handler struct { Store *store.Store }`
  - `func (h *Handler) Handle(evt any)` — despacha `*events.Message` (normaliza + InsertMessage + UpsertChat con preview) y `*events.HistorySync` (recorre conversaciones y persiste). Otros eventos se ignoran.

- [ ] **Step 1: Escribir el test que falla**

```go
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
```

- [ ] **Step 2: Correr y verlo fallar**

Run: `go test ./internal/ingest/ -run Handle`
Expected: FAIL (`Handler` indefinido).

- [ ] **Step 3: Implementar handler**

```go
// internal/ingest/handler.go
package ingest

import (
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
		// El historial llega como conversaciones con mensajes embebidos.
		// Se recorren e insertan los que podamos normalizar.
		for _, conv := range v.Data.GetConversations() {
			for _, hm := range conv.GetMessages() {
				wmsg := hm.GetMessage()
				if wmsg == nil {
					continue
				}
				h.onMessage(&events.Message{Info: parseHistoryInfo(conv, wmsg), Message: wmsg.GetMessage()})
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
```

> Nota de implementación para el ejecutor: `parseHistoryInfo` debe construir un `types.MessageInfo` a partir del `WebMessageInfo` del history sync (clave `Key.RemoteJID`, `Key.ID`, `MessageTimestamp`). Confirmar los getters exactos contra `go.mau.fi/whatsmeow/proto/waWeb` en godoc al implementar; el resto del handler ya queda fijado por este test. `chatType` devuelve `"group"` si el JID termina en `@g.us`, si no `"individual"`. `previewText` devuelve `m.Text` o, si está vacío y hay media, `"["+m.Type+"]"`.

```go
// (mismo archivo) helpers deterministas, ya cubiertos indirectamente:
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
```

> El cuerpo de `parseHistoryInfo` (rama HistorySync) NO está cubierto por el test de esta tarea y se valida en la verificación manual de la Task 8 (al linkear llega historial real). Implementarlo siguiendo la nota anterior.

- [ ] **Step 4: Correr tests (verde)**

Run: `go test ./internal/ingest/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ingest/handler.go internal/ingest/handler_test.go
git commit -m "feat: handler de ingesta de mensajes e historial al store"
```

---

### Task 6: IPC — servidor (daemon) y cliente (MCP)

**Files:**
- Create: `internal/ipc/types.go`
- Create: `internal/ipc/server.go`
- Create: `internal/ipc/client.go`
- Test: `internal/ipc/ipc_test.go`

**Interfaces:**
- Produces (tipos compartidos):
  - `type Status struct { Connected bool `json:"connected"`; NeedsQR bool `json:"needs_qr"`; JID string `json:"jid"` }`
  - `type DownloadRequest struct { Chat string `json:"chat"`; DestFolder string `json:"dest_folder"`; Types []string `json:"types"`; Limit int `json:"limit"` }`
  - `type SavedFile struct { Path string `json:"path"`; Mimetype string `json:"mimetype"`; Size int64 `json:"size"` }`
  - `type DownloadResult struct { Files []SavedFile `json:"files"`; Errors []string `json:"errors"` }`
- Produces (servidor): `type Backend interface { Status() Status; Download(ctx, DownloadRequest) (DownloadResult, error) }` y `func NewServer(token string, b Backend) http.Handler`.
- Produces (cliente): `type Client struct { BaseURL, Token string; HTTP *http.Client }`, con `func (c *Client) Status(ctx) (Status, error)` y `func (c *Client) Download(ctx, DownloadRequest) (DownloadResult, error)`.

- [ ] **Step 1: Escribir el test que falla**

```go
// internal/ipc/ipc_test.go
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
```

- [ ] **Step 2: Correr y verlo fallar**

Run: `go test ./internal/ipc/`
Expected: FAIL (símbolos indefinidos).

- [ ] **Step 3: Implementar tipos, servidor y cliente**

```go
// internal/ipc/types.go
package ipc

type Status struct {
	Connected bool   `json:"connected"`
	NeedsQR   bool   `json:"needs_qr"`
	JID       string `json:"jid"`
}

type DownloadRequest struct {
	Chat       string   `json:"chat"`
	DestFolder string   `json:"dest_folder"`
	Types      []string `json:"types"`
	Limit      int      `json:"limit"`
}

type SavedFile struct {
	Path     string `json:"path"`
	Mimetype string `json:"mimetype"`
	Size     int64  `json:"size"`
}

type DownloadResult struct {
	Files  []SavedFile `json:"files"`
	Errors []string    `json:"errors"`
}
```

```go
// internal/ipc/server.go
package ipc

import (
	"context"
	"encoding/json"
	"net/http"
)

type Backend interface {
	Status() Status
	Download(ctx context.Context, r DownloadRequest) (DownloadResult, error)
}

func NewServer(token string, b Backend) http.Handler {
	mux := http.NewServeMux()
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Token") != token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
	mux.HandleFunc("/status", auth(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, b.Status())
	}))
	mux.HandleFunc("/download", auth(func(w http.ResponseWriter, r *http.Request) {
		var req DownloadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := b.Download(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, res)
	}))
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
```

```go
// internal/ipc/client.go
package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	var st Status
	err := c.do(ctx, http.MethodGet, "/status", nil, &st)
	return st, err
}

func (c *Client) Download(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
	var res DownloadResult
	err := c.do(ctx, http.MethodPost, "/download", req, &res)
	return res, err
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("X-Token", c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon respondió %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
```

- [ ] **Step 4: Correr tests (verde)**

Run: `go test ./internal/ipc/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ipc/
git commit -m "feat: IPC localhost con token (servidor daemon + cliente MCP)"
```

---

### Task 7: Wrapper whatsmeow + Backend de descarga  **[verificación manual]**

**Files:**
- Create: `internal/wa/wa.go`
- Create: `internal/wa/backend.go`

**Interfaces:**
- Consumes: `*store.Store` (Task 3), `ingest.Handler` (Task 5), `ipc.*` (Task 6), tipos whatsmeow.
- Produces:
  - `type Client struct { ... }` con `func New(ctx, sessionDBPath string, h *ingest.Handler) (*Client, error)`, `func (c *Client) Start(ctx) error` (conecta; si no hay sesión, imprime QR), `func (c *Client) Status() ipc.Status`, `func (c *Client) DownloadAny(ctx, rawProto []byte) ([]byte, error)`.
  - `type Backend struct { Store *store.Store; WA *Client }` que implementa `ipc.Backend` (Status + Download: query media del store → DownloadAny → escribe archivos).

Esta tarea integra whatsmeow real; no se testea en CI. Las firmas externas están verificadas al 2026-06-19 (ver Global Constraints) pero **reconfirmar contra godoc al implementar**.

- [ ] **Step 1: Implementar el cliente whatsmeow**

```go
// internal/wa/wa.go
package wa

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"whatsapp-mcp/internal/ingest"
	"whatsapp-mcp/internal/ipc"
)

type Client struct {
	wm     *whatsmeow.Client
	mu     sync.Mutex
	needQR bool
}

func New(ctx context.Context, sessionDBPath string, h *ingest.Handler) (*Client, error) {
	dbLog := waLog.Stdout("Database", "WARN", true)
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", sessionDBPath)
	container, err := sqlstore.New(ctx, "sqlite", dsn, dbLog)
	if err != nil {
		return nil, err
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, err
	}
	wm := whatsmeow.NewClient(device, waLog.Stdout("Client", "WARN", true))
	c := &Client{wm: wm}
	wm.AddEventHandler(h.Handle)
	return c, nil
}

// Start conecta. Si el device no tiene sesión (Store.ID == nil),
// abre el canal de QR y lo imprime en la terminal del daemon.
func (c *Client) Start(ctx context.Context) error {
	if c.wm.Store.ID == nil {
		qrChan, err := c.wm.GetQRChannel(ctx)
		if err != nil {
			return err
		}
		if err := c.wm.Connect(); err != nil {
			return err
		}
		go func() {
			for evt := range qrChan {
				switch evt.Event {
				case "code":
					c.setNeedQR(true)
					fmt.Println("\nEscaneá este QR desde WhatsApp → Dispositivos vinculados:")
					qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				case "success":
					c.setNeedQR(false)
					fmt.Println("✓ Vinculado correctamente.")
				}
			}
		}()
		return nil
	}
	return c.wm.Connect()
}

func (c *Client) setNeedQR(v bool) { c.mu.Lock(); c.needQR = v; c.mu.Unlock() }

func (c *Client) Status() ipc.Status {
	c.mu.Lock()
	needQR := c.needQR
	c.mu.Unlock()
	st := ipc.Status{Connected: c.wm.IsConnected() && c.wm.IsLoggedIn(), NeedsQR: needQR}
	if c.wm.Store.ID != nil {
		st.JID = c.wm.Store.ID.String()
	}
	return st
}

// DownloadAny deserializa el proto guardado y descarga su media.
func (c *Client) DownloadAny(ctx context.Context, rawProto []byte) ([]byte, error) {
	var msg waE2E.Message
	if err := proto.Unmarshal(rawProto, &msg); err != nil {
		return nil, err
	}
	return c.wm.DownloadAny(ctx, &msg)
}

var _ = events.Message{} // mantiene el import si se referencia en evolución
```

> Reconfirmar al implementar: `c.wm.Store.ID` para saber si hay sesión, `IsConnected()`/`IsLoggedIn()`. Son API estable de whatsmeow; si algún nombre cambió, ajustarlo (godoc `pkg.go.dev/go.mau.fi/whatsmeow`).

- [ ] **Step 2: Implementar el Backend (ipc.Backend)**

```go
// internal/wa/backend.go
package wa

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"whatsapp-mcp/internal/ipc"
	"whatsapp-mcp/internal/store"
)

type Backend struct {
	Store *store.Store
	WA    *Client
}

func (b *Backend) Status() ipc.Status { return b.WA.Status() }

func (b *Backend) Download(ctx context.Context, req ipc.DownloadRequest) (ipc.DownloadResult, error) {
	chats, err := b.Store.ResolveChats(req.Chat)
	if err != nil {
		return ipc.DownloadResult{}, err
	}
	if len(chats) == 0 {
		return ipc.DownloadResult{}, fmt.Errorf("no encontré el chat %q", req.Chat)
	}
	if len(chats) > 1 {
		return ipc.DownloadResult{}, fmt.Errorf("%q es ambiguo (%d coincidencias); usá el JID exacto", req.Chat, len(chats))
	}
	limit := req.Limit
	if limit == 0 {
		limit = 50
	}
	media, err := b.Store.ListMedia(chats[0].JID, req.Types, limit)
	if err != nil {
		return ipc.DownloadResult{}, err
	}
	if err := os.MkdirAll(req.DestFolder, 0o755); err != nil {
		return ipc.DownloadResult{}, err
	}
	var res ipc.DownloadResult
	for _, m := range media {
		data, err := b.WA.DownloadAny(ctx, m.RawProto)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", m.ID, err))
			continue
		}
		name := fileName(m.ID, m.Media.Filename, m.Media.Mimetype)
		path := filepath.Join(req.DestFolder, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", m.ID, err))
			continue
		}
		res.Files = append(res.Files, ipc.SavedFile{Path: path, Mimetype: m.Media.Mimetype, Size: int64(len(data))})
	}
	return res, nil
}

func fileName(id, original, mimetype string) string {
	if original != "" {
		return original
	}
	ext := ".bin"
	if exts, _ := mime.ExtensionsByType(mimetype); len(exts) > 0 {
		ext = exts[0]
	}
	return id + ext
}

var _ ipc.Backend = (*Backend)(nil)
```

- [ ] **Step 3: Verificar que compila**

Run: `go build ./...`
Expected: compila sin errores. (Si algún símbolo de whatsmeow cambió de nombre, ajustarlo según godoc — ver nota.)

- [ ] **Step 4: Commit**

```bash
git add internal/wa/
git commit -m "feat: wrapper whatsmeow (conexión, QR, status) y backend de descarga"
```

---

### Task 8: Binario daemon  **[verificación manual]**

**Files:**
- Create: `cmd/daemon/main.go`
- Create: `config.example.json`

**Interfaces:**
- Consumes: `config` (Task 1), `store` (Task 2/3), `ingest.Handler` (Task 5), `wa.Client`/`wa.Backend` (Task 7), `ipc.NewServer` (Task 6).

- [ ] **Step 1: Implementar el daemon**

```go
// cmd/daemon/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"whatsapp-mcp/internal/config"
	"whatsapp-mcp/internal/ingest"
	"whatsapp-mcp/internal/ipc"
	"whatsapp-mcp/internal/store"
	"whatsapp-mcp/internal/wa"
)

func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(cfg.MessagesDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	h := &ingest.Handler{Store: st}
	client, err := wa.New(ctx, cfg.SessionDBPath(), h)
	if err != nil {
		log.Fatalf("whatsmeow: %v", err)
	}
	if err := client.Start(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}

	backend := &wa.Backend{Store: st, WA: client}
	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", cfg.IPCPort), Handler: ipc.NewServer(cfg.IPCToken, backend)}
	go func() {
		log.Printf("IPC escuchando en %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ipc: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("apagando…")
	srv.Close()
}
```

```json
// config.example.json  (copiar a config.json y poner un token aleatorio)
{
  "data_dir": "data",
  "ipc_port": 8377,
  "ipc_token": "CAMBIAME-por-un-token-aleatorio"
}
```

- [ ] **Step 2: Build**

Run: `go build -o whatsapp-daemon.exe ./cmd/daemon`
Expected: genera `whatsapp-daemon.exe`.

- [ ] **Step 3: Verificación manual — linkeo y captura**

```bash
cp config.example.json config.json
# editar config.json: poner un ipc_token aleatorio (ej: openssl rand -hex 16)
./whatsapp-daemon.exe
```
Esperado:
1. Se imprime un QR en la terminal.
2. Escanear desde WhatsApp → *Dispositivos vinculados*.
3. Aparece "✓ Vinculado correctamente." y empieza a sincronizar historial.
4. Verificar que se pobló la DB:
```bash
# en otra terminal
sqlite3 data/messages.db "SELECT count(*) FROM messages;"
```
Esperado: un número > 0 tras unos segundos. Dejar el daemon corriendo para la Task 10.

- [ ] **Step 4: Commit**

```bash
git add cmd/daemon/ config.example.json
git commit -m "feat: binario daemon (whatsmeow + ingesta + IPC + QR)"
```

---

### Task 9: Handlers de herramientas MCP

**Files:**
- Create: `internal/tools/tools.go`
- Test: `internal/tools/tools_test.go`

**Interfaces:**
- Consumes: `*store.Store` (Task 3), `ipc.Client`/tipos (Task 6).
- Produces:
  - `type DaemonClient interface { Status(ctx) (ipc.Status, error); Download(ctx, ipc.DownloadRequest) (ipc.DownloadResult, error) }` (lo implementa `*ipc.Client`).
  - `type Tools struct { Store *store.Store; Daemon DaemonClient }`
  - Handlers con firma go-sdk `func(ctx, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)`:
    `GetConnectionStatus`, `SearchContacts`, `ListChats`, `GetMessages`, `ListMedia`, `DownloadMedia`.
  - Structs In/Out por handler (definidos abajo).

- [ ] **Step 1: Escribir el test que falla** (cubre los handlers que leen del store)

```go
// internal/tools/tools_test.go
package tools

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"whatsapp-mcp/internal/model"
	"whatsapp-mcp/internal/store"
)

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
	return &Tools{Store: s}
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
```

- [ ] **Step 2: Correr y verlo fallar**

Run: `go test ./internal/tools/`
Expected: FAIL (símbolos indefinidos).

- [ ] **Step 3: Implementar los handlers**

```go
// internal/tools/tools.go
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
	if len(chats) == 1 {
		return chats[0].JID, nil, nil
	}
	var cands []ChatDTO
	for _, c := range chats {
		cands = append(cands, toChatDTO(c))
	}
	return "", cands, nil
}
```

- [ ] **Step 4: Correr tests (verde)**

Run: `go test ./internal/tools/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tools/
git commit -m "feat: handlers de las herramientas MCP (lectura + delegación de descarga)"
```

---

### Task 10: Binario servidor MCP  **[verificación manual]**

**Files:**
- Create: `cmd/mcp/main.go`

**Interfaces:**
- Consumes: `config` (Task 1), `store` (Task 2), `ipc.Client` (Task 6), `tools.*` (Task 9), go-sdk `mcp`.

- [ ] **Step 1: Implementar el servidor MCP**

```go
// cmd/mcp/main.go
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"whatsapp-mcp/internal/config"
	"whatsapp-mcp/internal/ipc"
	"whatsapp-mcp/internal/store"
	"whatsapp-mcp/internal/tools"
)

func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	st, err := store.Open(cfg.MessagesDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	daemon := &ipc.Client{BaseURL: cfg.BaseURL(), Token: cfg.IPCToken, HTTP: http.DefaultClient}
	t := &tools.Tools{Store: st, Daemon: daemon}

	server := mcp.NewServer(&mcp.Implementation{Name: "whatsapp-mcp", Version: "v0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "get_connection_status", Description: "Estado de la conexión a WhatsApp (conectado, necesita QR, número vinculado)"}, t.GetConnectionStatus)
	mcp.AddTool(server, &mcp.Tool{Name: "search_contacts", Description: "Buscar contactos/chats por nombre o teléfono; devuelve candidatos con su JID"}, t.SearchContacts)
	mcp.AddTool(server, &mcp.Tool{Name: "list_chats", Description: "Listar chats recientes con vista previa del último mensaje"}, t.ListChats)
	mcp.AddTool(server, &mcp.Tool{Name: "get_messages", Description: "Traer mensajes de un chat (por nombre o JID). Si el nombre es ambiguo devuelve candidatos"}, t.GetMessages)
	mcp.AddTool(server, &mcp.Tool{Name: "list_media", Description: "Listar adjuntos (imagen/audio/video/documento/sticker) disponibles en un chat"}, t.ListMedia)
	mcp.AddTool(server, &mcp.Tool{Name: "download_media", Description: "Descargar adjuntos de un chat a una carpeta destino; devuelve las rutas guardadas"}, t.DownloadMedia)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Build**

Run: `go build -o whatsapp-mcp.exe ./cmd/mcp`
Expected: genera `whatsapp-mcp.exe`.

- [ ] **Step 3: Verificación manual con Claude**

Con el daemon de la Task 8 corriendo y vinculado, registrar el MCP en el cliente (ver README, Task 11) y pedir a Claude:
1. "¿Cuál es el estado de conexión de WhatsApp?" → debe responder `connected: true` con tu JID.
2. "Buscá el contacto <nombre>" → devuelve candidatos.
3. "Resumime los últimos mensajes con <nombre>" → trae mensajes y resume.
4. "Descargá los archivos de ese chat a `C:\temp\wa`" → crea la carpeta y guarda archivos; verificar en disco.

- [ ] **Step 4: Commit**

```bash
git add cmd/mcp/
git commit -m "feat: binario servidor MCP (registro de herramientas + stdio)"
```

---

### Task 11: README y configuración del cliente MCP

**Files:**
- Create: `README.md`

- [ ] **Step 1: Escribir el README**

Contenido (adaptar rutas a la instalación real):

````markdown
# whatsapp-mcp

MCP en Go para leer WhatsApp (como WhatsApp Web, vía QR) y descargar adjuntos. Solo lectura.

## Requisitos
- Go 1.26+ (`winget install GoLang.Go`)
- Windows (probado), sin compilador C (SQLite es puro Go)

## Build
```powershell
go build -o whatsapp-daemon.exe ./cmd/daemon
go build -o whatsapp-mcp.exe ./cmd/mcp
```

## Configuración
Copiá `config.example.json` a `config.json` y poné un token aleatorio en `ipc_token`.

## Linkeo (una sola vez)
```powershell
.\whatsapp-daemon.exe
```
Escaneá el QR desde WhatsApp → *Dispositivos vinculados*. Dejá el daemon corriendo
en segundo plano (o configuralo en el Programador de tareas para que arranque con Windows):
es quien mantiene la sesión y captura los mensajes.

## Registrar el MCP en Claude
En la config de MCP del cliente (ej. Claude Desktop / Claude Code), agregá:
```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "C:\\dev\\mcp\\whatsapp-mcp\\whatsapp-mcp.exe",
      "cwd": "C:\\dev\\mcp\\whatsapp-mcp"
    }
  }
}
```
> `cwd` debe ser la carpeta del proyecto para que encuentre `config.json` y `data/`.

## Herramientas
- `get_connection_status` · `search_contacts` · `list_chats` · `get_messages` · `list_media` · `download_media`

## Privacidad
`data/messages.db` guarda el texto de tus mensajes en claro, sólo en tu máquina.
Uso no oficial del protocolo de WhatsApp; usalo con moderación.
````

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: README con build, linkeo y registro del MCP"
```

---

## Self-Review (completado por el autor del plan)

**1. Cobertura del spec:**
- §4 arquitectura (daemon + MCP + SQLite) → Tasks 8, 10, 2.
- §5 QR/linking → Task 7 (GetQRChannel + qrterminal), Task 8 (verificación manual).
- §6 storage (2 DBs, modernc) → Task 2 (messages.db), Task 7 (session.db vía sqlstore).
- §6 ingesta (Message/HistorySync/Contact) → Tasks 4, 5. *(Nota: contactos se pueblan vía `*events.Contact`/pushname; la rama no está testeada y se completa en la verificación manual de la Task 8 — ver más abajo.)*
- §7 las 6 herramientas → Task 9 + Task 10.
- §8 flujos → cubiertos por get_messages + download_media.
- §9 errores (no linkeado / daemon caído / ambigüedad) → Task 9 (mensajes de error) + Task 7 (ambigüedad en Download).
- §10 IPC token → Task 6.
- §11 testing → tests en Tasks 1–6, 9; manual en 8, 10.

**2. Placeholders:** El único cuerpo no escrito por completo es `parseHistoryInfo` (Task 5) y la ingesta de `*events.Contact`, ambos marcados como verificación manual con instrucciones precisas. No hay "TBD" en código testeable.

**3. Consistencia de tipos:** `model.Message`/`Chat`/`Contact`, firmas del store (`GetMessages(jid,limit)`, `ListMedia(jid,types,limit)`, `ResolveChats`), `ipc.Status`/`DownloadRequest`/`DownloadResult`/`SavedFile`, y las firmas de handlers go-sdk son consistentes entre tareas.

**Gap conocido a resolver durante la implementación (no bloqueante):** poblar `contacts` desde `*events.Contact`/`PushName` en el handler (Task 5). Añadir un `case *events.Contact:` en `Handle` que haga `UpsertContact`. Se deja como ajuste menor verificado a mano porque depende de los nombres exactos de campos del evento (confirmar en godoc).
