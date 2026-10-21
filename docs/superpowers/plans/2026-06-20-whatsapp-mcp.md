# WhatsApp MCP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Construir un servidor MCP en Go puro que conecta a WhatsApp Web (protocolo multi-device) y expone herramientas de solo lectura al asistente de IA para leer mensajes y descargar adjuntos.

**Architecture:** Único binario Go con tres capas: `whatsapp` (cliente whatsmeow), `store` (SQLite via database/sql), y `mcp` (servidor mcp-go via stdio). Las capas se comunican a través de interfaces definidas en `store`, permitiendo testear el store con SQLite en memoria y los tools MCP con mocks.

**Tech Stack:** Go 1.22+, `go.mau.fi/whatsmeow`, `github.com/mark3labs/mcp-go`, `github.com/glebarez/go-sqlite` (SQLite puro Go sin CGO, registra como "sqlite3"), `github.com/mdp/qrterminal/v3`, `gopkg.in/yaml.v3`, `google.golang.org/protobuf/proto`

---

## Mapa de archivos

| Archivo | Responsabilidad |
|---------|----------------|
| `go.mod` / `go.sum` | Módulo y dependencias |
| `config.yaml` | Configuración de usuario |
| `cmd/main.go` | Punto de entrada; carga config, inicializa y conecta capas |
| `internal/config/config.go` | Struct `Config`, función `Load(path)` |
| `internal/store/store.go` | Tipos de modelo (`Chat`, `Message`, `Attachment`) e interfaces (`ChatStore`, `MessageStore`, `AttachmentStore`) |
| `internal/store/db.go` | `DB` struct, `Open(path)`, ejecución del schema SQL |
| `internal/store/chats.go` | Implementación de `ChatStore` |
| `internal/store/chats_test.go` | Tests de ChatStore con SQLite en memoria |
| `internal/store/messages.go` | Implementación de `MessageStore` |
| `internal/store/messages_test.go` | Tests de MessageStore con SQLite en memoria |
| `internal/store/attachments.go` | Implementación de `AttachmentStore` |
| `internal/whatsapp/client.go` | `WAClient`: init whatsmeow, QR, phone link, status |
| `internal/whatsapp/handlers.go` | Handlers de eventos WA → persiste en store |
| `internal/mcp/server.go` | `MCPServer`: registra tools y lanza `server.ServeStdio` |
| `internal/mcp/tools_connection.go` | Handlers: `get_connection_status`, `connect_qr`, `connect_phone` |
| `internal/mcp/tools_messages.go` | Handlers: `list_chats`, `get_messages`, `get_message`, `search_messages` |
| `internal/mcp/tools_attachments.go` | Handler: `download_attachment` |

---

## Task 1: Scaffold del proyecto

**Files:**
- Create: `go.mod`
- Create: `config.yaml`
- Create: `internal/config/config.go`

- [ ] **Step 1.1: Inicializar módulo Go**

```bash
cd c:\dev\mcp\whatsapp-mcp
go mod init whatsapp-mcp
```

Resultado esperado: se crea `go.mod` con `module whatsapp-mcp` y `go 1.22.x`.

- [ ] **Step 1.2: Crear estructura de directorios**

```bash
mkdir -p cmd internal/config internal/store internal/whatsapp internal/mcp
```

- [ ] **Step 1.3: Crear config.yaml**

```yaml
media_dir: "C:/whatsapp-media"
db_path: "./data"
log_level: "info"
```

- [ ] **Step 1.4: Escribir internal/config/config.go**

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MediaDir string `yaml:"media_dir"`
	DBPath   string `yaml:"db_path"`
	LogLevel string `yaml:"log_level"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.MediaDir == "" {
		return nil, fmt.Errorf("media_dir is required")
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "./data"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	return &cfg, nil
}
```

- [ ] **Step 1.5: Instalar dependencias**

```bash
go get gopkg.in/yaml.v3
go get github.com/glebarez/go-sqlite
go get go.mau.fi/whatsmeow
go get github.com/mark3labs/mcp-go
go get github.com/mdp/qrterminal/v3
go get google.golang.org/protobuf
```

Verificar: `go mod tidy` no produce errores.

- [ ] **Step 1.6: Commit**

```bash
git add go.mod go.sum config.yaml internal/config/config.go
git commit -m "feat: scaffold project and config package"
```

---

## Task 2: Modelos e interfaces del store

**Files:**
- Create: `internal/store/store.go`

- [ ] **Step 2.1: Escribir internal/store/store.go**

```go
package store

import "time"

// --- Modelos ---

type Chat struct {
	JID                   string
	Name                  string
	LastMessageTimestamp  time.Time
	UnreadCount           int
}

type Message struct {
	ID             string
	ChatJID        string
	SenderJID      string
	Timestamp      time.Time
	Type           string // text | image | video | audio | document | sticker
	Body           string
	HasAttachment  bool
	IsDownloaded   bool
	RawMediaProto  []byte // marshaled proto para download posterior
}

type Attachment struct {
	MessageID string
	FilePath  string
	MimeType  string
	FileSize  int64
	SHA256    string
}

// --- Interfaces ---

type ChatStore interface {
	UpsertChat(chat Chat) error
	ListChats() ([]Chat, error)
}

type MessageStore interface {
	InsertMessage(msg Message) error
	GetMessages(chatJID string, limit, offset int, from, to int64) ([]Message, error)
	GetMessage(id string) (*Message, error)
	SearchMessages(query, chatJID string) ([]Message, error)
}

type AttachmentStore interface {
	UpsertAttachment(att Attachment) error
	GetAttachment(messageID string) (*Attachment, error)
	MarkDownloaded(messageID, filePath string) error
}
```

- [ ] **Step 2.2: Commit**

```bash
git add internal/store/store.go
git commit -m "feat: define store models and interfaces"
```

---

## Task 3: Inicialización de base de datos

**Files:**
- Create: `internal/store/db.go`

- [ ] **Step 3.1: Escribir internal/store/db.go**

```go
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS chats (
	jid                   TEXT PRIMARY KEY,
	name                  TEXT NOT NULL DEFAULT '',
	last_message_timestamp INTEGER NOT NULL DEFAULT 0,
	unread_count          INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS messages (
	id             TEXT PRIMARY KEY,
	chat_jid       TEXT NOT NULL,
	sender_jid     TEXT NOT NULL,
	timestamp      INTEGER NOT NULL,
	type           TEXT NOT NULL,
	body           TEXT NOT NULL DEFAULT '',
	has_attachment INTEGER NOT NULL DEFAULT 0,
	is_downloaded  INTEGER NOT NULL DEFAULT 0,
	raw_media_proto BLOB,
	FOREIGN KEY (chat_jid) REFERENCES chats(jid)
);

CREATE INDEX IF NOT EXISTS idx_messages_chat_ts ON messages(chat_jid, timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_body    ON messages(body);

CREATE TABLE IF NOT EXISTS attachments (
	message_id TEXT PRIMARY KEY,
	file_path  TEXT NOT NULL DEFAULT '',
	mime_type  TEXT NOT NULL DEFAULT '',
	file_size  INTEGER NOT NULL DEFAULT 0,
	sha256     TEXT NOT NULL DEFAULT '',
	FOREIGN KEY (message_id) REFERENCES messages(id)
);
`

type DB struct {
	sql *sql.DB
}

func Open(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	path := filepath.Join(dir, "messages.db")
	db, err := sql.Open("sqlite3", "file:"+path+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open messages db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &DB{sql: db}, nil
}

func (d *DB) Close() error { return d.sql.Close() }

// Exportar sql para uso en implementaciones del paquete
func (d *DB) SQL() *sql.DB { return d.sql }
```

- [ ] **Step 3.2: Commit**

```bash
git add internal/store/db.go
git commit -m "feat: database initialization and schema"
```

---

## Task 4: Chat store

**Files:**
- Create: `internal/store/chats.go`
- Create: `internal/store/chats_test.go`

- [ ] **Step 4.1: Escribir el test primero (internal/store/chats_test.go)**

```go
package store_test

import (
	"fmt"
	"testing"
	"time"

	"whatsapp-mcp/internal/store"
)

func newTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUpsertAndListChats(t *testing.T) {
	db := newTestDB(t)
	cs := store.NewChatStore(db)

	chat := store.Chat{
		JID:                  "5491112345678@s.whatsapp.net",
		Name:                 "Juan",
		LastMessageTimestamp: time.Unix(1700000000, 0),
		UnreadCount:          3,
	}

	if err := cs.UpsertChat(chat); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	chats, err := cs.ListChats()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(chats) != 1 {
		t.Fatalf("expected 1 chat, got %d", len(chats))
	}
	if chats[0].Name != "Juan" {
		t.Errorf("expected name Juan, got %s", chats[0].Name)
	}
	if chats[0].UnreadCount != 3 {
		t.Errorf("expected unread 3, got %d", chats[0].UnreadCount)
	}
}

func TestUpsertChatUpdatesExisting(t *testing.T) {
	db := newTestDB(t)
	cs := store.NewChatStore(db)

	chat := store.Chat{JID: "5491112345678@s.whatsapp.net", Name: "Juan"}
	_ = cs.UpsertChat(chat)

	chat.Name = "Juan Actualizado"
	_ = cs.UpsertChat(chat)

	chats, _ := cs.ListChats()
	if chats[0].Name != "Juan Actualizado" {
		t.Errorf("expected updated name, got %s", chats[0].Name)
	}
}
```

- [ ] **Step 4.2: Ejecutar test para verificar que falla**

```bash
go test ./internal/store/... -run TestUpsertAndListChats -v
```

Resultado esperado: error de compilación (NewChatStore no definido).

- [ ] **Step 4.3: Escribir internal/store/chats.go**

```go
package store

import (
	"fmt"
	"time"
)

type chatStore struct{ db *DB }

func NewChatStore(db *DB) ChatStore { return &chatStore{db: db} }

func (s *chatStore) UpsertChat(chat Chat) error {
	_, err := s.db.SQL().Exec(`
		INSERT INTO chats (jid, name, last_message_timestamp, unread_count)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = excluded.name,
			last_message_timestamp = excluded.last_message_timestamp,
			unread_count = excluded.unread_count`,
		chat.JID,
		chat.Name,
		chat.LastMessageTimestamp.Unix(),
		chat.UnreadCount,
	)
	if err != nil {
		return fmt.Errorf("upsert chat: %w", err)
	}
	return nil
}

func (s *chatStore) ListChats() ([]Chat, error) {
	rows, err := s.db.SQL().Query(`
		SELECT jid, name, last_message_timestamp, unread_count
		FROM chats
		ORDER BY last_message_timestamp DESC`)
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var c Chat
		var ts int64
		if err := rows.Scan(&c.JID, &c.Name, &ts, &c.UnreadCount); err != nil {
			return nil, fmt.Errorf("scan chat: %w", err)
		}
		c.LastMessageTimestamp = time.Unix(ts, 0)
		chats = append(chats, c)
	}
	return chats, rows.Err()
}
```

- [ ] **Step 4.4: Ejecutar tests**

```bash
go test ./internal/store/... -run TestUpsertAndListChats -v
go test ./internal/store/... -run TestUpsertChatUpdatesExisting -v
```

Resultado esperado: PASS ambos tests.

- [ ] **Step 4.5: Commit**

```bash
git add internal/store/chats.go internal/store/chats_test.go
git commit -m "feat: chat store with upsert and list"
```

---

## Task 5: Message store

**Files:**
- Create: `internal/store/messages.go`
- Create: `internal/store/messages_test.go`

- [ ] **Step 5.1: Escribir internal/store/messages_test.go**

```go
package store_test

import (
	"testing"
	"time"

	"whatsapp-mcp/internal/store"
)

func seedChat(t *testing.T, cs store.ChatStore, jid string) {
	t.Helper()
	_ = cs.UpsertChat(store.Chat{JID: jid, Name: "Test"})
}

func TestInsertAndGetMessages(t *testing.T) {
	db := newTestDB(t)
	cs := store.NewChatStore(db)
	ms := store.NewMessageStore(db)

	chatJID := "5491112345678@s.whatsapp.net"
	seedChat(t, cs, chatJID)

	msg := store.Message{
		ID:        "msg1",
		ChatJID:   chatJID,
		SenderJID: chatJID,
		Timestamp: time.Unix(1700000000, 0),
		Type:      "text",
		Body:      "Hola mundo",
	}
	if err := ms.InsertMessage(msg); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := ms.GetMessage("msg1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Body != "Hola mundo" {
		t.Errorf("expected body 'Hola mundo', got %q", got.Body)
	}
}

func TestGetMessagesWithPagination(t *testing.T) {
	db := newTestDB(t)
	cs := store.NewChatStore(db)
	ms := store.NewMessageStore(db)

	chatJID := "group1@g.us"
	seedChat(t, cs, chatJID)

	for i := range 5 {
		_ = ms.InsertMessage(store.Message{
			ID:        fmt.Sprintf("m%d", i),
			ChatJID:   chatJID,
			SenderJID: chatJID,
			Timestamp: time.Unix(int64(1700000000+i), 0),
			Type:      "text",
			Body:      fmt.Sprintf("msg %d", i),
		})
	}

	msgs, err := ms.GetMessages(chatJID, 2, 1, 0, 0)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestSearchMessages(t *testing.T) {
	db := newTestDB(t)
	cs := store.NewChatStore(db)
	ms := store.NewMessageStore(db)

	chatJID := "5491112345678@s.whatsapp.net"
	seedChat(t, cs, chatJID)
	_ = ms.InsertMessage(store.Message{ID: "a", ChatJID: chatJID, SenderJID: chatJID, Timestamp: time.Now(), Type: "text", Body: "reunión mañana"})
	_ = ms.InsertMessage(store.Message{ID: "b", ChatJID: chatJID, SenderJID: chatJID, Timestamp: time.Now(), Type: "text", Body: "hasta luego"})

	results, err := ms.SearchMessages("reunión", "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].ID != "a" {
		t.Errorf("expected message a, got %+v", results)
	}
}
```

El import de `"fmt"` ya está incluido en el bloque de código anterior.

- [ ] **Step 5.2: Ejecutar tests para verificar que fallan**

```bash
go test ./internal/store/... -run "TestInsertAndGetMessages|TestGetMessagesWithPagination|TestSearchMessages" -v
```

Resultado esperado: error de compilación (NewMessageStore no definido).

- [ ] **Step 5.3: Escribir internal/store/messages.go**

```go
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type messageStore struct{ db *DB }

func NewMessageStore(db *DB) MessageStore { return &messageStore{db: db} }

func (s *messageStore) InsertMessage(msg Message) error {
	_, err := s.db.SQL().Exec(`
		INSERT OR IGNORE INTO messages
			(id, chat_jid, sender_jid, timestamp, type, body, has_attachment, is_downloaded, raw_media_proto)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ChatJID, msg.SenderJID, msg.Timestamp.Unix(),
		msg.Type, msg.Body,
		boolToInt(msg.HasAttachment),
		boolToInt(msg.IsDownloaded),
		msg.RawMediaProto,
	)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

func (s *messageStore) GetMessage(id string) (*Message, error) {
	row := s.db.SQL().QueryRow(`
		SELECT id, chat_jid, sender_jid, timestamp, type, body, has_attachment, is_downloaded, raw_media_proto
		FROM messages WHERE id = ?`, id)
	msg, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return msg, err
}

func (s *messageStore) GetMessages(chatJID string, limit, offset int, from, to int64) ([]Message, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
		SELECT id, chat_jid, sender_jid, timestamp, type, body, has_attachment, is_downloaded, raw_media_proto
		FROM messages
		WHERE chat_jid = ?`
	args := []any{chatJID}
	if from > 0 {
		query += " AND timestamp >= ?"
		args = append(args, from)
	}
	if to > 0 {
		query += " AND timestamp <= ?"
		args = append(args, to)
	}
	query += " ORDER BY timestamp ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.SQL().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

func (s *messageStore) SearchMessages(query, chatJID string) ([]Message, error) {
	q := `
		SELECT id, chat_jid, sender_jid, timestamp, type, body, has_attachment, is_downloaded, raw_media_proto
		FROM messages WHERE body LIKE ?`
	args := []any{"%" + query + "%"}
	if chatJID != "" {
		q += " AND chat_jid = ?"
		args = append(args, chatJID)
	}
	q += " ORDER BY timestamp DESC LIMIT 200"

	rows, err := s.db.SQL().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

// --- helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func scanMessage(row scanner) (*Message, error) {
	var m Message
	var ts int64
	var hasAtt, isDl int
	err := row.Scan(&m.ID, &m.ChatJID, &m.SenderJID, &ts, &m.Type, &m.Body, &hasAtt, &isDl, &m.RawMediaProto)
	if err != nil {
		return nil, err
	}
	m.Timestamp = time.Unix(ts, 0)
	m.HasAttachment = hasAtt == 1
	m.IsDownloaded = isDl == 1
	return &m, nil
}

func scanMessages(rows *sql.Rows) ([]Message, error) {
	var msgs []Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, *m)
	}
	return msgs, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
```

- [ ] **Step 5.4: Ejecutar todos los tests del store**

```bash
go test ./internal/store/... -v
```

Resultado esperado: todos en PASS.

- [ ] **Step 5.5: Commit**

```bash
git add internal/store/messages.go internal/store/messages_test.go
git commit -m "feat: message store with insert, get, pagination and search"
```

---

## Task 6: Attachment store

**Files:**
- Create: `internal/store/attachments.go`

- [ ] **Step 6.1: Escribir internal/store/attachments.go**

```go
package store

import (
	"database/sql"
	"errors"
	"fmt"
)

type attachmentStore struct{ db *DB }

func NewAttachmentStore(db *DB) AttachmentStore { return &attachmentStore{db: db} }

func (s *attachmentStore) UpsertAttachment(att Attachment) error {
	_, err := s.db.SQL().Exec(`
		INSERT INTO attachments (message_id, file_path, mime_type, file_size, sha256)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(message_id) DO UPDATE SET
			file_path = excluded.file_path,
			mime_type  = excluded.mime_type,
			file_size  = excluded.file_size,
			sha256     = excluded.sha256`,
		att.MessageID, att.FilePath, att.MimeType, att.FileSize, att.SHA256,
	)
	if err != nil {
		return fmt.Errorf("upsert attachment: %w", err)
	}
	return nil
}

func (s *attachmentStore) GetAttachment(messageID string) (*Attachment, error) {
	var a Attachment
	err := s.db.SQL().QueryRow(`
		SELECT message_id, file_path, mime_type, file_size, sha256
		FROM attachments WHERE message_id = ?`, messageID).
		Scan(&a.MessageID, &a.FilePath, &a.MimeType, &a.FileSize, &a.SHA256)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get attachment: %w", err)
	}
	return &a, nil
}

func (s *attachmentStore) MarkDownloaded(messageID, filePath string) error {
	_, err := s.db.SQL().Exec(
		`UPDATE messages SET is_downloaded = 1 WHERE id = ?`, messageID)
	if err != nil {
		return fmt.Errorf("mark downloaded: %w", err)
	}
	_, err = s.db.SQL().Exec(
		`UPDATE attachments SET file_path = ? WHERE message_id = ?`, filePath, messageID)
	if err != nil {
		return fmt.Errorf("update attachment path: %w", err)
	}
	return nil
}
```

- [ ] **Step 6.2: Ejecutar tests**

```bash
go test ./internal/store/... -v
```

Resultado esperado: todos en PASS (sin tests nuevos para attachments; la compilación verifica correctness).

- [ ] **Step 6.3: Commit**

```bash
git add internal/store/attachments.go
git commit -m "feat: attachment store"
```

---

## Task 7: WhatsApp client

**Files:**
- Create: `internal/whatsapp/client.go`

- [ ] **Step 7.1: Escribir internal/whatsapp/client.go**

```go
package whatsapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	_ "github.com/glebarez/go-sqlite"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type Status string

const (
	StatusDisconnected  Status = "disconnected"
	StatusConnecting    Status = "connecting"
	StatusWaitingQR     Status = "waiting_qr"
	StatusWaitingPhone  Status = "waiting_phone_code"
	StatusConnected     Status = "connected"
)

type Client struct {
	wa     *whatsmeow.Client
	status atomic.Value // stores Status
}

func New(dbDir string) (*Client, error) {
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	sessionPath := filepath.Join(dbDir, "session.db")
	dbLog := waLog.Stdout("WAStore", "ERROR", true)
	container, err := sqlstore.New("sqlite3", "file:"+sessionPath+"?_foreign_keys=on", dbLog)
	if err != nil {
		return nil, fmt.Errorf("open session db: %w", err)
	}
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		return nil, fmt.Errorf("get device store: %w", err)
	}
	clientLog := waLog.Stdout("WAClient", "ERROR", true)
	wa := whatsmeow.NewClient(deviceStore, clientLog)

	c := &Client{wa: wa}
	c.status.Store(StatusDisconnected)
	return c, nil
}

// IsLoggedIn returns true if a session exists (no QR needed).
func (c *Client) IsLoggedIn() bool {
	return c.wa.Store.ID != nil
}

// ConnectExisting reconnects using a saved session (no QR).
func (c *Client) ConnectExisting() error {
	if !c.IsLoggedIn() {
		return fmt.Errorf("no saved session; use ConnectQR or ConnectPhone")
	}
	c.status.Store(StatusConnecting)
	if err := c.wa.Connect(); err != nil {
		c.status.Store(StatusDisconnected)
		return fmt.Errorf("connect: %w", err)
	}
	c.status.Store(StatusConnected)
	return nil
}

// ConnectQR starts a new session via QR code.
// Returns the QR code as ASCII art text to display to the user.
func (c *Client) ConnectQR(ctx context.Context) (string, error) {
	if c.IsLoggedIn() {
		return "", fmt.Errorf("already logged in; disconnect first")
	}
	c.status.Store(StatusConnecting)

	qrChan, _ := c.wa.GetQRChannel(ctx)

	go func() {
		if err := c.wa.Connect(); err != nil {
			c.status.Store(StatusDisconnected)
		}
	}()

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			c.status.Store(StatusWaitingQR)
			return qrToASCII(evt.Code), nil
		case "success":
			c.status.Store(StatusConnected)
			return "", nil
		case "timeout":
			c.status.Store(StatusDisconnected)
			return "", fmt.Errorf("QR code timed out; try again")
		default:
			// "err" or unknown — keep waiting
		}
	}
	return "", fmt.Errorf("QR channel closed unexpectedly")
}

// ConnectPhone starts a new session via phone number linking.
// phoneNumber must be in international format without +, e.g. "5491112345678".
// Returns the 8-character pairing code to enter on the phone.
func (c *Client) ConnectPhone(ctx context.Context, phoneNumber string) (string, error) {
	if c.IsLoggedIn() {
		return "", fmt.Errorf("already logged in; disconnect first")
	}
	c.status.Store(StatusConnecting)

	go func() {
		if err := c.wa.Connect(); err != nil {
			c.status.Store(StatusDisconnected)
		}
	}()

	code, err := c.wa.PairPhone(phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		c.status.Store(StatusDisconnected)
		return "", fmt.Errorf("pair phone: %w", err)
	}
	c.status.Store(StatusWaitingPhone)
	return code, nil
}

func (c *Client) Status() Status {
	return c.status.Load().(Status)
}

// WA exposes the underlying whatsmeow client for event handler registration.
func (c *Client) WA() *whatsmeow.Client { return c.wa }

// JID returns this device's JID once connected.
func (c *Client) JID() *types.JID {
	return c.wa.Store.ID
}

// SetStatus is called by event handlers when connection state changes.
func (c *Client) SetStatus(s Status) { c.status.Store(s) }

// DownloadMedia downloads a media message given its serialized proto bytes and type.
func (c *Client) DownloadMedia(rawProto []byte, msgType string) ([]byte, error) {
	return downloadByType(c.wa, rawProto, msgType)
}

// --- helpers ---

func qrToASCII(code string) string {
	var b strings.Builder
	qrterminal.GenerateHalfBlock(code, qrterminal.L, &b)
	return b.String()
}
```

- [ ] **Step 7.2: Crear internal/whatsapp/download.go con la lógica de descarga por tipo**

```go
package whatsapp

import (
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func downloadByType(wa *whatsmeow.Client, rawProto []byte, msgType string) ([]byte, error) {
	switch msgType {
	case "image":
		var m waE2E.ImageMessage
		if err := proto.Unmarshal(rawProto, &m); err != nil {
			return nil, fmt.Errorf("unmarshal image proto: %w", err)
		}
		return wa.Download(&m)
	case "video":
		var m waE2E.VideoMessage
		if err := proto.Unmarshal(rawProto, &m); err != nil {
			return nil, fmt.Errorf("unmarshal video proto: %w", err)
		}
		return wa.Download(&m)
	case "audio":
		var m waE2E.AudioMessage
		if err := proto.Unmarshal(rawProto, &m); err != nil {
			return nil, fmt.Errorf("unmarshal audio proto: %w", err)
		}
		return wa.Download(&m)
	case "document":
		var m waE2E.DocumentMessage
		if err := proto.Unmarshal(rawProto, &m); err != nil {
			return nil, fmt.Errorf("unmarshal document proto: %w", err)
		}
		return wa.Download(&m)
	case "sticker":
		var m waE2E.StickerMessage
		if err := proto.Unmarshal(rawProto, &m); err != nil {
			return nil, fmt.Errorf("unmarshal sticker proto: %w", err)
		}
		return wa.Download(&m)
	default:
		return nil, fmt.Errorf("unsupported media type: %s", msgType)
	}
}
```

- [ ] **Step 7.3: Verificar compilación**

```bash
go build ./internal/whatsapp/...
```

Resultado esperado: sin errores.

- [ ] **Step 7.4: Commit**

```bash
git add internal/whatsapp/client.go internal/whatsapp/download.go
git commit -m "feat: WhatsApp client with QR, phone linking and media download"
```

---

## Task 8: WhatsApp event handlers

**Files:**
- Create: `internal/whatsapp/handlers.go`

- [ ] **Step 8.1: Escribir internal/whatsapp/handlers.go**

```go
package whatsapp

import (
	"log"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"whatsapp-mcp/internal/store"
)

// RegisterHandlers wires the whatsmeow event handler that persists messages to the store.
func RegisterHandlers(c *Client, cs store.ChatStore, ms store.MessageStore, as store.AttachmentStore) {
	c.wa.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Connected:
			c.SetStatus(StatusConnected)
		case *events.LoggedOut:
			c.SetStatus(StatusDisconnected)
		case *events.Disconnected:
			c.SetStatus(StatusDisconnected)
		case *events.Message:
			handleMessage(v, cs, ms, as)
		case *events.HistorySync:
			handleHistorySync(v, cs, ms)
		}
	})
}

func handleMessage(evt *events.Message, cs store.ChatStore, ms store.MessageStore, as store.AttachmentStore) {
	info := evt.Info
	chatJID := info.Chat.String()

	_ = cs.UpsertChat(store.Chat{
		JID:                  chatJID,
		Name:                 info.PushName,
		LastMessageTimestamp: info.Timestamp,
	})

	msg := store.Message{
		ID:        info.ID,
		ChatJID:   chatJID,
		SenderJID: info.Sender.String(),
		Timestamp: info.Timestamp,
	}
	extractContent(evt.Message, &msg)

	if err := ms.InsertMessage(msg); err != nil {
		log.Printf("insert message %s: %v", info.ID, err)
		return
	}

	if msg.HasAttachment {
		_ = as.UpsertAttachment(store.Attachment{
			MessageID: msg.ID,
			MimeType:  mimeFromType(msg.Type),
		})
	}
}

func handleHistorySync(evt *events.HistorySync, cs store.ChatStore, ms store.MessageStore) {
	for _, conv := range evt.Data.GetConversations() {
		chatJID := conv.GetId()
		name := conv.GetName()
		if name == "" {
			name = chatJID
		}
		_ = cs.UpsertChat(store.Chat{
			JID:  chatJID,
			Name: name,
		})

		for _, histMsg := range conv.GetMessages() {
			webMsg := histMsg.GetMessage()
			if webMsg == nil {
				continue
			}
			info := webMsg.GetKey()
			msgProto := webMsg.GetMessage()
			if msgProto == nil {
				continue
			}
			ts := time.Unix(int64(webMsg.GetMessageTimestamp()), 0)
			msg := store.Message{
				ID:        info.GetId(),
				ChatJID:   chatJID,
				SenderJID: senderFromKey(info),
				Timestamp: ts,
			}
			extractContent(msgProto, &msg)
			_ = ms.InsertMessage(msg)
		}
	}
}

func extractContent(m *waE2E.Message, msg *store.Message) {
	switch {
	case m.GetConversation() != "":
		msg.Type = "text"
		msg.Body = m.GetConversation()
	case m.GetExtendedTextMessage() != nil:
		msg.Type = "text"
		msg.Body = m.GetExtendedTextMessage().GetText()
	case m.GetImageMessage() != nil:
		msg.Type = "image"
		msg.Body = m.GetImageMessage().GetCaption()
		msg.HasAttachment = true
		msg.RawMediaProto, _ = proto.Marshal(m.GetImageMessage())
	case m.GetVideoMessage() != nil:
		msg.Type = "video"
		msg.Body = m.GetVideoMessage().GetCaption()
		msg.HasAttachment = true
		msg.RawMediaProto, _ = proto.Marshal(m.GetVideoMessage())
	case m.GetAudioMessage() != nil:
		msg.Type = "audio"
		msg.HasAttachment = true
		msg.RawMediaProto, _ = proto.Marshal(m.GetAudioMessage())
	case m.GetDocumentMessage() != nil:
		msg.Type = "document"
		msg.Body = m.GetDocumentMessage().GetFileName()
		msg.HasAttachment = true
		msg.RawMediaProto, _ = proto.Marshal(m.GetDocumentMessage())
	case m.GetStickerMessage() != nil:
		msg.Type = "sticker"
		msg.HasAttachment = true
		msg.RawMediaProto, _ = proto.Marshal(m.GetStickerMessage())
	default:
		msg.Type = "text"
		msg.Body = ""
	}
}

func senderFromKey(key interface{ GetFromMe() bool; GetParticipant() string; GetRemoteJid() string }) string {
	if key.GetFromMe() {
		return key.GetRemoteJid()
	}
	if p := key.GetParticipant(); p != "" {
		return p
	}
	return key.GetRemoteJid()
}

func mimeFromType(t string) string {
	switch t {
	case "image":
		return "image/jpeg"
	case "video":
		return "video/mp4"
	case "audio":
		return "audio/ogg"
	case "document":
		return "application/octet-stream"
	case "sticker":
		return "image/webp"
	}
	return ""
}

func mimeToExt(mime string) string {
	m := map[string]string{
		"image/jpeg":          ".jpg",
		"image/png":           ".png",
		"image/webp":          ".webp",
		"video/mp4":           ".mp4",
		"audio/ogg":           ".ogg",
		"audio/mp4":           ".m4a",
		"application/pdf":     ".pdf",
		"application/octet-stream": "",
	}
	if ext, ok := m[mime]; ok {
		return ext
	}
	return ""
}

// MimeToExt is exported for use in tools_attachments.go
func MimeToExt(mime string) string { return mimeToExt(mime) }
```

- [ ] **Step 8.2: Verificar compilación**

```bash
go build ./internal/whatsapp/...
```

Resultado esperado: sin errores.

- [ ] **Step 8.3: Commit**

```bash
git add internal/whatsapp/handlers.go
git commit -m "feat: WhatsApp event handlers for messages and history sync"
```

---

## Task 9: MCP server + tools de conexión

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/tools_connection.go`

- [ ] **Step 9.1: Escribir internal/mcp/server.go**

```go
package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"whatsapp-mcp/internal/store"
	"whatsapp-mcp/internal/whatsapp"
)

// Start registers all MCP tools and blocks serving over stdio.
func Start(waClient *whatsapp.Client, db *store.DB) error {
	cs := store.NewChatStore(db)
	ms := store.NewMessageStore(db)
	as := store.NewAttachmentStore(db)

	s := server.NewMCPServer("whatsapp-mcp", "1.0.0")

	registerConnectionTools(s, waClient)
	registerMessageTools(s, cs, ms)
	registerAttachmentTools(s, waClient, ms, as)

	return server.ServeStdio(s)
}
```

- [ ] **Step 9.2: Escribir internal/mcp/tools_connection.go**

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"whatsapp-mcp/internal/whatsapp"
)

func registerConnectionTools(s *server.MCPServer, c *whatsapp.Client) {
	// get_connection_status
	s.AddTool(
		mcp.NewTool("get_connection_status",
			mcp.WithDescription("Devuelve el estado actual de la conexión con WhatsApp"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			status := c.Status()
			out, _ := json.Marshal(map[string]string{"status": string(status)})
			return mcp.NewToolResultText(string(out)), nil
		},
	)

	// connect_qr
	s.AddTool(
		mcp.NewTool("connect_qr",
			mcp.WithDescription("Inicia la conexión con WhatsApp vía código QR. Devuelve el QR como texto ASCII para escanearlo con el teléfono."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			qr, err := c.ConnectQR(ctx)
			if err != nil {
				return nil, fmt.Errorf("connect_qr: %w", err)
			}
			if qr == "" {
				return mcp.NewToolResultText("Ya conectado exitosamente"), nil
			}
			return mcp.NewToolResultText("Escanea este código QR con WhatsApp en tu teléfono:\n\n" + qr), nil
		},
	)

	// connect_phone
	s.AddTool(
		mcp.NewTool("connect_phone",
			mcp.WithDescription("Inicia la conexión con WhatsApp vía número de teléfono. Devuelve un código de vinculación de 8 caracteres para ingresar en la app."),
			mcp.WithString("phone_number",
				mcp.Required(),
				mcp.Description("Número de teléfono en formato internacional sin +, ej: 5491112345678"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			phone, ok := req.Params.Arguments["phone_number"].(string)
			if !ok || phone == "" {
				return nil, fmt.Errorf("phone_number is required")
			}
			code, err := c.ConnectPhone(ctx, phone)
			if err != nil {
				return nil, fmt.Errorf("connect_phone: %w", err)
			}
			return mcp.NewToolResultText(fmt.Sprintf(
				"Código de vinculación: %s\nIngresa este código en WhatsApp > Dispositivos vinculados > Vincular con número de teléfono",
				code,
			)), nil
		},
	)
}
```

- [ ] **Step 9.3: Verificar compilación**

```bash
go build ./internal/mcp/...
```

- [ ] **Step 9.4: Commit**

```bash
git add internal/mcp/server.go internal/mcp/tools_connection.go
git commit -m "feat: MCP server and connection tools"
```

---

## Task 10: MCP tools de mensajes

**Files:**
- Create: `internal/mcp/tools_messages.go`

- [ ] **Step 10.1: Escribir internal/mcp/tools_messages.go**

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"whatsapp-mcp/internal/store"
)

func registerMessageTools(s *server.MCPServer, cs store.ChatStore, ms store.MessageStore) {
	// list_chats
	s.AddTool(
		mcp.NewTool("list_chats",
			mcp.WithDescription("Lista todos los chats de WhatsApp (individuales y grupos) ordenados por actividad reciente"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			chats, err := cs.ListChats()
			if err != nil {
				return nil, fmt.Errorf("list_chats: %w", err)
			}
			type chatDTO struct {
				JID                  string `json:"jid"`
				Name                 string `json:"name"`
				LastMessageTimestamp string `json:"last_message_timestamp"`
				UnreadCount          int    `json:"unread_count"`
			}
			result := make([]chatDTO, len(chats))
			for i, c := range chats {
				result[i] = chatDTO{
					JID:                  c.JID,
					Name:                 c.Name,
					LastMessageTimestamp: c.LastMessageTimestamp.Format(time.RFC3339),
					UnreadCount:          c.UnreadCount,
				}
			}
			out, _ := json.Marshal(result)
			return mcp.NewToolResultText(string(out)), nil
		},
	)

	// get_messages
	s.AddTool(
		mcp.NewTool("get_messages",
			mcp.WithDescription("Devuelve mensajes de un chat dado con soporte de paginación y filtro por fecha"),
			mcp.WithString("chat_jid", mcp.Required(), mcp.Description("JID del chat, ej: 5491112345678@s.whatsapp.net o grupo@g.us")),
			mcp.WithNumber("limit", mcp.Description("Máximo de mensajes a devolver (default 100)")),
			mcp.WithNumber("offset", mcp.Description("Offset para paginación (default 0)")),
			mcp.WithNumber("from_timestamp", mcp.Description("Filtrar mensajes desde este timestamp Unix (opcional)")),
			mcp.WithNumber("to_timestamp", mcp.Description("Filtrar mensajes hasta este timestamp Unix (opcional)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			chatJID, ok := req.Params.Arguments["chat_jid"].(string)
			if !ok || chatJID == "" {
				return nil, fmt.Errorf("chat_jid is required")
			}
			limit := int(floatArg(req, "limit"))
			offset := int(floatArg(req, "offset"))
			from := int64(floatArg(req, "from_timestamp"))
			to := int64(floatArg(req, "to_timestamp"))

			msgs, err := ms.GetMessages(chatJID, limit, offset, from, to)
			if err != nil {
				return nil, fmt.Errorf("get_messages: %w", err)
			}
			out, _ := json.Marshal(messagesToDTO(msgs))
			return mcp.NewToolResultText(string(out)), nil
		},
	)

	// get_message
	s.AddTool(
		mcp.NewTool("get_message",
			mcp.WithDescription("Obtiene un mensaje específico por su ID"),
			mcp.WithString("message_id", mcp.Required(), mcp.Description("ID del mensaje")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, ok := req.Params.Arguments["message_id"].(string)
			if !ok || id == "" {
				return nil, fmt.Errorf("message_id is required")
			}
			msg, err := ms.GetMessage(id)
			if err != nil {
				return nil, fmt.Errorf("get_message: %w", err)
			}
			if msg == nil {
				return mcp.NewToolResultText(`{"error":"message not found"}`), nil
			}
			out, _ := json.Marshal(messageToDTO(*msg))
			return mcp.NewToolResultText(string(out)), nil
		},
	)

	// search_messages
	s.AddTool(
		mcp.NewTool("search_messages",
			mcp.WithDescription("Busca mensajes por texto en todos los chats o en uno específico"),
			mcp.WithString("query", mcp.Required(), mcp.Description("Texto a buscar")),
			mcp.WithString("chat_jid", mcp.Description("JID del chat para limitar la búsqueda (opcional)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, ok := req.Params.Arguments["query"].(string)
			if !ok || query == "" {
				return nil, fmt.Errorf("query is required")
			}
			chatJID, _ := req.Params.Arguments["chat_jid"].(string)
			msgs, err := ms.SearchMessages(query, chatJID)
			if err != nil {
				return nil, fmt.Errorf("search_messages: %w", err)
			}
			out, _ := json.Marshal(messagesToDTO(msgs))
			return mcp.NewToolResultText(string(out)), nil
		},
	)
}

// --- DTOs ---

type messageDTO struct {
	ID            string     `json:"id"`
	ChatJID       string     `json:"chat_jid"`
	SenderJID     string     `json:"sender_jid"`
	Timestamp     string     `json:"timestamp"`
	Type          string     `json:"type"`
	Body          string     `json:"body"`
	HasAttachment bool       `json:"has_attachment"`
	IsDownloaded  bool       `json:"is_downloaded"`
}

func messageToDTO(m store.Message) messageDTO {
	return messageDTO{
		ID:            m.ID,
		ChatJID:       m.ChatJID,
		SenderJID:     m.SenderJID,
		Timestamp:     m.Timestamp.Format(time.RFC3339),
		Type:          m.Type,
		Body:          m.Body,
		HasAttachment: m.HasAttachment,
		IsDownloaded:  m.IsDownloaded,
	}
}

func messagesToDTO(msgs []store.Message) []messageDTO {
	result := make([]messageDTO, len(msgs))
	for i, m := range msgs {
		result[i] = messageToDTO(m)
	}
	return result
}

func floatArg(req mcp.CallToolRequest, key string) float64 {
	v, _ := req.Params.Arguments[key].(float64)
	return v
}
```

- [ ] **Step 10.2: Verificar compilación**

```bash
go build ./internal/mcp/...
```

- [ ] **Step 10.3: Commit**

```bash
git add internal/mcp/tools_messages.go
git commit -m "feat: MCP tools for listing chats, reading and searching messages"
```

---

## Task 11: MCP tool de descarga de adjuntos

**Files:**
- Create: `internal/mcp/tools_attachments.go`

- [ ] **Step 11.1: Escribir internal/mcp/tools_attachments.go**

```go
package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"whatsapp-mcp/internal/store"
	"whatsapp-mcp/internal/whatsapp"
)

func registerAttachmentTools(
	s *server.MCPServer,
	c *whatsapp.Client,
	ms store.MessageStore,
	as store.AttachmentStore,
) {
	s.AddTool(
		mcp.NewTool("download_attachment",
			mcp.WithDescription("Descarga el adjunto de un mensaje (imagen, video, audio, documento) a la carpeta local configurada y devuelve la ruta en disco"),
			mcp.WithString("message_id", mcp.Required(), mcp.Description("ID del mensaje con adjunto")),
		),
		makeDownloadHandler(c, ms, as),
	)
}

func makeDownloadHandler(
	c *whatsapp.Client,
	ms store.MessageStore,
	as store.AttachmentStore,
) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		msgID, ok := req.Params.Arguments["message_id"].(string)
		if !ok || msgID == "" {
			return nil, fmt.Errorf("message_id is required")
		}

		msg, err := ms.GetMessage(msgID)
		if err != nil {
			return nil, fmt.Errorf("get message: %w", err)
		}
		if msg == nil {
			return nil, fmt.Errorf("message %s not found", msgID)
		}
		if !msg.HasAttachment {
			return nil, fmt.Errorf("message %s has no attachment", msgID)
		}

		// If already downloaded, return existing path
		att, _ := as.GetAttachment(msgID)
		if att != nil && att.FilePath != "" {
			out, _ := json.Marshal(map[string]string{"file_path": att.FilePath, "mime_type": att.MimeType})
			return mcp.NewToolResultText(string(out)), nil
		}

		// Download from WhatsApp
		data, err := c.DownloadMedia(msg.RawMediaProto, msg.Type)
		if err != nil {
			return nil, fmt.Errorf("download media: %w", err)
		}

		// Determine mime type and extension
		mime := guessMime(msg.Type, att)
		ext := whatsapp.MimeToExt(mime)

		// Save to media_dir/<chat_jid>/<message_id><ext>
		mediaDir := mediaDir() // resolved from env or default
		dir := filepath.Join(mediaDir, sanitizeJID(msg.ChatJID))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create media dir: %w", err)
		}
		filePath := filepath.Join(dir, msgID+ext)
		if err := os.WriteFile(filePath, data, 0o644); err != nil {
			return nil, fmt.Errorf("write file: %w", err)
		}

		// Compute SHA256
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])

		// Update store
		_ = as.UpsertAttachment(store.Attachment{
			MessageID: msgID,
			FilePath:  filePath,
			MimeType:  mime,
			FileSize:  int64(len(data)),
			SHA256:    hash,
		})
		_ = as.MarkDownloaded(msgID, filePath)

		out, _ := json.Marshal(map[string]interface{}{
			"file_path":  filePath,
			"mime_type":  mime,
			"file_size":  len(data),
			"sha256":     hash,
		})
		return mcp.NewToolResultText(string(out)), nil
	}
}

func guessMime(msgType string, att *store.Attachment) string {
	if att != nil && att.MimeType != "" {
		return att.MimeType
	}
	switch msgType {
	case "image":
		return "image/jpeg"
	case "video":
		return "video/mp4"
	case "audio":
		return "audio/ogg"
	case "document":
		return "application/octet-stream"
	case "sticker":
		return "image/webp"
	}
	return "application/octet-stream"
}

func sanitizeJID(jid string) string {
	// Remove characters unsafe for directory names
	safe := make([]byte, 0, len(jid))
	for _, b := range []byte(jid) {
		if b == '@' || b == '.' || b == '+' {
			b = '_'
		}
		safe = append(safe, b)
	}
	return string(safe)
}

// mediaDir reads WHATSAPP_MEDIA_DIR env var; falls back to ./media
func mediaDir() string {
	if d := os.Getenv("WHATSAPP_MEDIA_DIR"); d != "" {
		return d
	}
	return "./media"
}
```

- [ ] **Step 11.2: Verificar compilación**

```bash
go build ./internal/mcp/...
```

- [ ] **Step 11.3: Commit**

```bash
git add internal/mcp/tools_attachments.go
git commit -m "feat: MCP download_attachment tool"
```

---

## Task 12: Punto de entrada (main.go)

**Files:**
- Create: `cmd/main.go`

- [ ] **Step 12.1: Escribir cmd/main.go**

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"whatsapp-mcp/internal/config"
	mcpserver "whatsapp-mcp/internal/mcp"
	"whatsapp-mcp/internal/store"
	"whatsapp-mcp/internal/whatsapp"
)

func main() {
	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Set media dir for attachment downloads
	os.Setenv("WHATSAPP_MEDIA_DIR", cfg.MediaDir)

	// Open message store
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer db.Close()

	// Initialize WhatsApp client
	waClient, err := whatsapp.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("init whatsapp client: %v", err)
	}

	// Register event handlers (messages → store)
	cs := store.NewChatStore(db)
	ms := store.NewMessageStore(db)
	as := store.NewAttachmentStore(db)
	whatsapp.RegisterHandlers(waClient, cs, ms, as)

	// If already logged in, reconnect automatically
	if waClient.IsLoggedIn() {
		log.Println("Reconnecting saved session...")
		if err := waClient.ConnectExisting(); err != nil {
			log.Printf("reconnect failed: %v (use connect_qr or connect_phone via MCP)", err)
		}
	} else {
		log.Println("No saved session. Use connect_qr or connect_phone tool to authenticate.")
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		waClient.WA().Disconnect()
		os.Exit(0)
	}()

	// Start MCP server (blocks on stdio)
	log.Println("WhatsApp MCP server starting...")
	if err := mcpserver.Start(waClient, db); err != nil {
		log.Fatalf("mcp server: %v", err)
	}
}
```

- [ ] **Step 12.2: Verificar compilación completa**

```bash
go build ./...
```

Resultado esperado: sin errores. Se genera el ejecutable en el directorio actual.

- [ ] **Step 12.3: Ejecutar tests completos**

```bash
go test ./... -v
```

Resultado esperado: todos los tests en PASS.

- [ ] **Step 12.4: Commit**

```bash
git add cmd/main.go
git commit -m "feat: main entry point wiring all layers"
```

---

## Task 13: Build final y prueba de humo

**Files:** ninguno nuevo — verificación.

- [ ] **Step 13.1: Build de producción**

```bash
go build -o whatsapp-mcp.exe ./cmd/main.go
```

Resultado esperado: se crea `whatsapp-mcp.exe` sin errores.

- [ ] **Step 13.2: Crear carpeta de media**

```bash
mkdir -p C:/whatsapp-media
```

- [ ] **Step 13.3: Configurar Claude Desktop para usar el MCP**

Añadir al archivo `%APPDATA%\Claude\claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "C:\\dev\\mcp\\whatsapp-mcp\\whatsapp-mcp.exe",
      "args": ["C:\\dev\\mcp\\whatsapp-mcp\\config.yaml"]
    }
  }
}
```

- [ ] **Step 13.4: Reiniciar Claude Desktop y verificar que el MCP aparece**

En Claude Desktop, los tools `get_connection_status`, `connect_qr`, `connect_phone`, `list_chats`, `get_messages`, `get_message`, `search_messages`, `download_attachment` deben aparecer disponibles.

- [ ] **Step 13.5: Autenticarse con QR**

Pedirle a Claude: *"Usa el tool connect_qr para conectar WhatsApp"*

1. Claude llama a `connect_qr`
2. Aparece el QR en ASCII en la conversación
3. Abrir WhatsApp en el teléfono → Dispositivos vinculados → Vincular dispositivo → escanear QR
4. Llamar `get_connection_status` → debe devolver `{"status":"connected"}`

- [ ] **Step 13.6: Verificar lectura de mensajes**

Pedirle a Claude: *"Lista mis chats de WhatsApp"* → debe devolver JSON con los chats sincronizados.

- [ ] **Step 13.7: Commit final**

```bash
git add .
git commit -m "chore: add whatsapp-mcp.exe to gitignore"
```

Crear `.gitignore`:
```
whatsapp-mcp.exe
data/
C:/whatsapp-media/
*.db
```

```bash
git add .gitignore
git commit -m "chore: gitignore for build artifacts and data files"
```
