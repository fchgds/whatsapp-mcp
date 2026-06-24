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
		ON CONFLICT(jid) DO UPDATE SET
			name=CASE WHEN excluded.name!='' THEN excluded.name ELSE chats.name END,
			type=excluded.type,
			last_message_text=excluded.last_message_text,
			last_message_ts=excluded.last_message_ts`,
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

// SyncContactNames toma un mapa jid→nombre de la store interna de whatsmeow
// y actualiza tanto contacts como chats que tengan name vacío.
func (s *Store) SyncContactNames(names map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for jid, name := range names {
		phone := jid
		if idx := len(jid) - len("@s.whatsapp.net"); idx > 0 && jid[idx:] == "@s.whatsapp.net" {
			phone = jid[:idx]
		}
		if _, err := tx.Exec(`INSERT INTO contacts (jid,name,phone) VALUES (?,?,?)
			ON CONFLICT(jid) DO UPDATE SET name=excluded.name, phone=excluded.phone`,
			jid, name, phone); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE chats SET name=? WHERE jid=? AND (name IS NULL OR name='')`,
			name, jid); err != nil {
			return err
		}
	}
	return tx.Commit()
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

func scanChats(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]model.Chat, error) {
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

func scanMessages(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]model.Message, error) {
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
