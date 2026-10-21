import type { Database } from "bun:sqlite"
import type { Message, GetMessagesOpts } from "../types"

type MessageRow = {
  id: string; chat_jid: string; sender_jid: string; timestamp: number
  type: string; body: string; has_attachment: number; is_downloaded: number
  raw_media_proto: Buffer | null
}

function rowToMessage(r: MessageRow): Message {
  return {
    id: r.id, chatJID: r.chat_jid, senderJID: r.sender_jid, timestamp: r.timestamp,
    type: r.type as Message["type"], body: r.body,
    hasAttachment: r.has_attachment === 1, isDownloaded: r.is_downloaded === 1,
    rawMediaProto: r.raw_media_proto,
  }
}

const SELECT = `SELECT id, chat_jid, sender_jid, timestamp, type, body, has_attachment, is_downloaded, raw_media_proto FROM messages`

export function insertMessage(db: Database, msg: Message): void {
  db.run(
    `INSERT OR IGNORE INTO messages (id, chat_jid, sender_jid, timestamp, type, body, has_attachment, is_downloaded, raw_media_proto)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [msg.id, msg.chatJID, msg.senderJID, msg.timestamp, msg.type, msg.body,
     msg.hasAttachment ? 1 : 0, msg.isDownloaded ? 1 : 0, msg.rawMediaProto ?? null]
  )
}

export function getMessage(db: Database, id: string): Message | null {
  const row = db.query(`${SELECT} WHERE id = ?`).get(id) as MessageRow | null
  return row ? rowToMessage(row) : null
}

export function getMessages(db: Database, opts: GetMessagesOpts): Message[] {
  const { chatJID, limit = 100, offset = 0, fromTimestamp, toTimestamp } = opts
  let sql = `${SELECT} WHERE chat_jid = ?`
  const params: (string | number)[] = [chatJID]
  if (fromTimestamp !== undefined) { sql += " AND timestamp >= ?"; params.push(fromTimestamp) }
  if (toTimestamp !== undefined)   { sql += " AND timestamp <= ?"; params.push(toTimestamp) }
  sql += " ORDER BY timestamp ASC LIMIT ? OFFSET ?"
  params.push(limit, offset)
  return (db.query(sql).all(...params) as MessageRow[]).map(rowToMessage)
}

export function searchMessages(db: Database, query: string, chatJID?: string): Message[] {
  let sql = `${SELECT} WHERE body LIKE ?`
  const params: (string | number)[] = [`%${query}%`]
  if (chatJID) { sql += " AND chat_jid = ?"; params.push(chatJID) }
  sql += " ORDER BY timestamp DESC LIMIT 200"
  return (db.query(sql).all(...params) as MessageRow[]).map(rowToMessage)
}
