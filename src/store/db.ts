import { Database } from "bun:sqlite"
import { mkdirSync } from "node:fs"
import { dirname } from "node:path"

const SCHEMA = `
CREATE TABLE IF NOT EXISTS chats (
  jid                    TEXT PRIMARY KEY,
  name                   TEXT NOT NULL DEFAULT '',
  last_message_timestamp INTEGER NOT NULL DEFAULT 0,
  unread_count           INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS messages (
  id               TEXT PRIMARY KEY,
  chat_jid         TEXT NOT NULL,
  sender_jid       TEXT NOT NULL,
  timestamp        INTEGER NOT NULL,
  type             TEXT NOT NULL,
  body             TEXT NOT NULL DEFAULT '',
  has_attachment   INTEGER NOT NULL DEFAULT 0,
  is_downloaded    INTEGER NOT NULL DEFAULT 0,
  raw_media_proto  BLOB,
  FOREIGN KEY (chat_jid) REFERENCES chats(jid)
);

CREATE INDEX IF NOT EXISTS idx_messages_chat_ts ON messages(chat_jid, timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_body    ON messages(body);

CREATE TABLE IF NOT EXISTS contacts (
  jid           TEXT PRIMARY KEY,
  lid           TEXT NOT NULL DEFAULT '',
  name          TEXT NOT NULL DEFAULT '',
  notify        TEXT NOT NULL DEFAULT '',
  verified_name TEXT NOT NULL DEFAULT '',
  img_url       TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_contacts_name ON contacts(name);
CREATE INDEX IF NOT EXISTS idx_contacts_notify ON contacts(notify);
CREATE INDEX IF NOT EXISTS idx_contacts_verified_name ON contacts(verified_name);

CREATE TABLE IF NOT EXISTS attachments (
  message_id TEXT PRIMARY KEY,
  file_path  TEXT NOT NULL DEFAULT '',
  mime_type  TEXT NOT NULL DEFAULT '',
  file_size  INTEGER NOT NULL DEFAULT 0,
  sha256     TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (message_id) REFERENCES messages(id)
);
`

export function openDB(filePath: string): Database {
  if (filePath !== ":memory:") {
    mkdirSync(dirname(filePath), { recursive: true })
  }
  const db = new Database(filePath, { create: true })
  db.run("PRAGMA foreign_keys = ON")
  db.run("PRAGMA journal_mode = WAL")
  applySchema(db)
  return db
}

export function applySchema(db: Database): void {
  db.run(SCHEMA)
}
