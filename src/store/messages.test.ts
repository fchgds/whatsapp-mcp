import { describe, it, expect } from "bun:test"
import { Database } from "bun:sqlite"
import { applySchema } from "./db"
import { upsertChat } from "./chats"
import { insertMessage, getMessages, getMessage, searchMessages } from "./messages"
import type { Message } from "../types"

const CHAT = "5491112345678@s.whatsapp.net"

function makeTestDB(): Database {
  const db = new Database(":memory:")
  db.run("PRAGMA foreign_keys = ON")
  applySchema(db)
  upsertChat(db, { jid: CHAT, name: "Test", lastMessageTimestamp: 0, unreadCount: 0 })
  return db
}

function makeMsg(overrides: Partial<Message> = {}): Message {
  return {
    id: "msg1", chatJID: CHAT, senderJID: CHAT, timestamp: 1700000000,
    type: "text", body: "Hola", hasAttachment: false, isDownloaded: false, rawMediaProto: null,
    ...overrides,
  }
}

describe("messages store", () => {
  it("inserts and retrieves a message by id", () => {
    const db = makeTestDB()
    insertMessage(db, makeMsg({ body: "Hola mundo" }))
    const msg = getMessage(db, "msg1")
    expect(msg).not.toBeNull()
    expect(msg!.body).toBe("Hola mundo")
    expect(msg!.hasAttachment).toBe(false)
  })

  it("returns null for unknown id", () => {
    const db = makeTestDB()
    expect(getMessage(db, "nope")).toBeNull()
  })

  it("paginates with limit and offset", () => {
    const db = makeTestDB()
    for (let i = 0; i < 5; i++) {
      insertMessage(db, makeMsg({ id: `m${i}`, timestamp: 1700000000 + i, body: `msg ${i}` }))
    }
    const page = getMessages(db, { chatJID: CHAT, limit: 2, offset: 1 })
    expect(page).toHaveLength(2)
    expect(page[0].id).toBe("m1")
    expect(page[1].id).toBe("m2")
  })

  it("filters by fromTimestamp and toTimestamp", () => {
    const db = makeTestDB()
    insertMessage(db, makeMsg({ id: "a", timestamp: 100 }))
    insertMessage(db, makeMsg({ id: "b", timestamp: 200 }))
    insertMessage(db, makeMsg({ id: "c", timestamp: 300 }))
    const msgs = getMessages(db, { chatJID: CHAT, fromTimestamp: 150, toTimestamp: 250 })
    expect(msgs).toHaveLength(1)
    expect(msgs[0].id).toBe("b")
  })

  it("searches messages by body text", () => {
    const db = makeTestDB()
    insertMessage(db, makeMsg({ id: "a", body: "reunión mañana" }))
    insertMessage(db, makeMsg({ id: "b", body: "hasta luego" }))
    const results = searchMessages(db, "reunión")
    expect(results).toHaveLength(1)
    expect(results[0].id).toBe("a")
  })

  it("searches within a specific chat", () => {
    const db = makeTestDB()
    const OTHER = "other@s.whatsapp.net"
    upsertChat(db, { jid: OTHER, name: "Other", lastMessageTimestamp: 0, unreadCount: 0 })
    insertMessage(db, makeMsg({ id: "a", body: "hello" }))
    insertMessage(db, makeMsg({ id: "b", chatJID: OTHER, senderJID: OTHER, body: "hello" }))
    const results = searchMessages(db, "hello", CHAT)
    expect(results).toHaveLength(1)
    expect(results[0].chatJID).toBe(CHAT)
  })
})
