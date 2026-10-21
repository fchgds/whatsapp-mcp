import type { Database } from "bun:sqlite"
import type { Attachment } from "../types"

type AttachmentRow = {
  message_id: string; file_path: string; mime_type: string; file_size: number; sha256: string
}

function rowToAttachment(r: AttachmentRow): Attachment {
  return { messageID: r.message_id, filePath: r.file_path, mimeType: r.mime_type, fileSize: r.file_size, sha256: r.sha256 }
}

export function upsertAttachment(db: Database, att: Attachment): void {
  db.run(
    `INSERT INTO attachments (message_id, file_path, mime_type, file_size, sha256) VALUES (?, ?, ?, ?, ?)
     ON CONFLICT(message_id) DO UPDATE SET file_path=excluded.file_path, mime_type=excluded.mime_type, file_size=excluded.file_size, sha256=excluded.sha256`,
    [att.messageID, att.filePath, att.mimeType, att.fileSize, att.sha256]
  )
}

export function getAttachment(db: Database, messageID: string): Attachment | null {
  const row = db.query(
    `SELECT message_id, file_path, mime_type, file_size, sha256 FROM attachments WHERE message_id = ?`
  ).get(messageID) as AttachmentRow | null
  return row ? rowToAttachment(row) : null
}

export function markDownloaded(db: Database, messageID: string, filePath: string): void {
  db.run(`UPDATE messages SET is_downloaded = 1 WHERE id = ?`, [messageID])
  db.run(`UPDATE attachments SET file_path = ? WHERE message_id = ?`, [filePath, messageID])
}
