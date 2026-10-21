import type { Database } from "bun:sqlite"
import type { Contact } from "../types"

type ContactRow = {
  jid: string
  lid: string
  name: string
  notify: string
  verified_name: string
  img_url: string
  status: string
}

function rowToContact(row: ContactRow): Contact {
  return {
    jid: row.jid,
    lid: row.lid,
    name: row.name,
    notify: row.notify,
    verifiedName: row.verified_name,
    imgUrl: row.img_url,
    status: row.status,
  }
}

const SELECT = "SELECT jid, lid, name, notify, verified_name, img_url, status FROM contacts"

export function upsertContact(db: Database, contact: Contact): void {
  db.run(
    `INSERT INTO contacts (jid, lid, name, notify, verified_name, img_url, status)
     VALUES (?, ?, ?, ?, ?, ?, ?)
     ON CONFLICT(jid) DO UPDATE SET
       lid = COALESCE(NULLIF(excluded.lid, ''), contacts.lid),
       name = COALESCE(NULLIF(excluded.name, ''), contacts.name),
       notify = COALESCE(NULLIF(excluded.notify, ''), contacts.notify),
       verified_name = COALESCE(NULLIF(excluded.verified_name, ''), contacts.verified_name),
       img_url = COALESCE(NULLIF(excluded.img_url, ''), contacts.img_url),
       status = COALESCE(NULLIF(excluded.status, ''), contacts.status)`,
    [contact.jid, contact.lid, contact.name, contact.notify, contact.verifiedName, contact.imgUrl, contact.status]
  )
}

export function listContacts(db: Database, limit = 200): Contact[] {
  const rows = db.query(
    `${SELECT}
     ORDER BY COALESCE(NULLIF(name, ''), NULLIF(notify, ''), NULLIF(verified_name, ''), jid)
     LIMIT ?`
  ).all(limit) as ContactRow[]
  return rows.map(rowToContact)
}

export function searchContacts(db: Database, query: string, limit = 50): Contact[] {
  const like = `%${query}%`
  const rows = db.query(
    `${SELECT}
     WHERE jid LIKE ?
        OR lid LIKE ?
        OR name LIKE ?
        OR notify LIKE ?
        OR verified_name LIKE ?
        OR status LIKE ?
     ORDER BY COALESCE(NULLIF(name, ''), NULLIF(notify, ''), NULLIF(verified_name, ''), jid)
     LIMIT ?`
  ).all(like, like, like, like, like, like, limit) as ContactRow[]
  return rows.map(rowToContact)
}
