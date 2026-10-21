import type { Database } from "bun:sqlite"
import type { Chat } from "../types"

type ChatRow = {
  jid: string
  name: string
  last_message_timestamp: number
  unread_count: number
}

export function upsertChat(db: Database, chat: Chat): void {
  db.run(
    `INSERT INTO chats (jid, name, last_message_timestamp, unread_count)
     VALUES (?, ?, ?, ?)
     ON CONFLICT(jid) DO UPDATE SET
       name = excluded.name,
       last_message_timestamp = excluded.last_message_timestamp,
       unread_count = excluded.unread_count`,
    [chat.jid, chat.name, chat.lastMessageTimestamp, chat.unreadCount]
  )
}

export function listChats(db: Database): Chat[] {
  const rows = db.query(
    `SELECT jid, name, last_message_timestamp, unread_count
     FROM chats ORDER BY last_message_timestamp DESC`
  ).all() as ChatRow[]
  return rows.map((r) => ({
    jid: r.jid,
    name: r.name,
    lastMessageTimestamp: r.last_message_timestamp,
    unreadCount: r.unread_count,
  }))
}
