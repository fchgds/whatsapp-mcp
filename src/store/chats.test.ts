import { describe, it, expect } from "bun:test"
import { Database } from "bun:sqlite"
import { applySchema } from "./db"
import { upsertChat, listChats } from "./chats"
import type { Chat } from "../types"

function makeTestDB(): Database {
  const db = new Database(":memory:")
  db.run("PRAGMA foreign_keys = ON")
  applySchema(db)
  return db
}

describe("chats store", () => {
  it("inserts a chat and retrieves it", () => {
    const db = makeTestDB()
    const chat: Chat = { jid: "123@s.whatsapp.net", name: "Juan", lastMessageTimestamp: 1700000000, unreadCount: 3 }
    upsertChat(db, chat)
    const chats = listChats(db)
    expect(chats).toHaveLength(1)
    expect(chats[0].name).toBe("Juan")
    expect(chats[0].unreadCount).toBe(3)
    expect(chats[0].lastMessageTimestamp).toBe(1700000000)
  })

  it("updates an existing chat on conflict", () => {
    const db = makeTestDB()
    upsertChat(db, { jid: "123@s.whatsapp.net", name: "Viejo", lastMessageTimestamp: 0, unreadCount: 0 })
    upsertChat(db, { jid: "123@s.whatsapp.net", name: "Nuevo", lastMessageTimestamp: 1700000001, unreadCount: 5 })
    const chats = listChats(db)
    expect(chats).toHaveLength(1)
    expect(chats[0].name).toBe("Nuevo")
    expect(chats[0].unreadCount).toBe(5)
  })

  it("orders chats by lastMessageTimestamp descending", () => {
    const db = makeTestDB()
    upsertChat(db, { jid: "a@s.whatsapp.net", name: "A", lastMessageTimestamp: 100, unreadCount: 0 })
    upsertChat(db, { jid: "b@s.whatsapp.net", name: "B", lastMessageTimestamp: 200, unreadCount: 0 })
    const chats = listChats(db)
    expect(chats[0].jid).toBe("b@s.whatsapp.net")
    expect(chats[1].jid).toBe("a@s.whatsapp.net")
  })
})
