import { proto, type WAMessage } from "@whiskeysockets/baileys"
import type { Database } from "bun:sqlite"
import { waState, setReregisterCallback } from "./client"
import { upsertChat } from "../store/chats"
import { upsertContact } from "../store/contacts"
import { insertMessage } from "../store/messages"
import { upsertAttachment } from "../store/attachments"
import type { Message, MessageType } from "../types"
import type { Contact } from "@whiskeysockets/baileys/lib/Types/Contact"

export function registerHandlers(db: Database): void {
  // Guardamos db para re-registrar en reconexiones
  setReregisterCallback(() => attachHandlers(db))
  attachHandlers(db)
}

function attachHandlers(db: Database): void {
  const sock = waState.socket
  if (!sock) return // socket aún no disponible, se llamará desde createSocket()

  sock.ev.on("messages.upsert", ({ messages }) => {
    for (const msg of messages) {
      if (msg.message) handleMessage(db, msg)
    }
  })

  sock.ev.on("messaging-history.set", ({ chats, contacts, messages }) => {
    for (const contact of contacts) {
      handleContact(db, contact)
    }
    for (const chat of chats) {
      upsertChat(db, {
        jid: chat.id,
        name: (chat as { name?: string }).name ?? chat.id,
        lastMessageTimestamp: 0,
        unreadCount: 0,
      })
    }
    for (const msg of messages) {
      if (msg.message) handleMessage(db, msg)
    }
  })

  sock.ev.on("contacts.upsert", (contacts) => {
    for (const contact of contacts) {
      handleContact(db, contact)
    }
  })

  sock.ev.on("contacts.update", (contacts) => {
    for (const contact of contacts) {
      handleContact(db, contact)
    }
  })
}

function handleContact(db: Database, contact: Partial<Contact>): void {
  if (!contact.id) return
  upsertContact(db, {
    jid: contact.id,
    lid: contact.lid ?? "",
    name: contact.name ?? "",
    notify: contact.notify ?? "",
    verifiedName: contact.verifiedName ?? "",
    imgUrl: contact.imgUrl ?? "",
    status: contact.status ?? "",
  })
}

function chatNameFor(waMsg: WAMessage, chatJID: string): string {
  // En grupos (@g.us), pushName es el nombre del remitente, no del grupo
  // No lo usamos para evitar pisar el nombre real del grupo
  if (chatJID.endsWith("@g.us")) {
    return chatJID
  }
  return waMsg.pushName ?? chatJID
}

function handleMessage(db: Database, waMsg: WAMessage): void {
  const key = waMsg.key
  if (!key?.remoteJid || !waMsg.message) return

  const chatJID = key.remoteJid
  const senderJID = key.participant ?? key.remoteJid
  const timestamp = Number(waMsg.messageTimestamp ?? 0)

  // unreadCount no se rastrea en tiempo real por mensaje, siempre queda en 0
  upsertChat(db, { jid: chatJID, name: chatNameFor(waMsg, chatJID), lastMessageTimestamp: timestamp, unreadCount: 0 })

  const { type, body, hasAttachment, rawMediaProto } = extractContent(waMsg)
  const msg: Message = {
    id: key.id ?? "", chatJID, senderJID, timestamp, type, body, hasAttachment,
    isDownloaded: false, rawMediaProto,
  }
  insertMessage(db, msg)

  if (hasAttachment) {
    upsertAttachment(db, { messageID: key.id ?? "", filePath: "", mimeType: mimeFromType(type), fileSize: 0, sha256: "" })
  }
}

function extractContent(waMsg: WAMessage): {
  type: MessageType; body: string; hasAttachment: boolean; rawMediaProto: Buffer | null
} {
  const m = waMsg.message!
  const encodeProto = () => Buffer.from(proto.Message.encode(m).finish())

  if (m.conversation) return { type: "text", body: m.conversation, hasAttachment: false, rawMediaProto: null }
  if (m.extendedTextMessage?.text) return { type: "text", body: m.extendedTextMessage.text, hasAttachment: false, rawMediaProto: null }
  if (m.imageMessage)    return { type: "image",    body: m.imageMessage.caption ?? "",    hasAttachment: true, rawMediaProto: encodeProto() }
  if (m.videoMessage)    return { type: "video",    body: m.videoMessage.caption ?? "",    hasAttachment: true, rawMediaProto: encodeProto() }
  if (m.audioMessage)    return { type: "audio",    body: "",                              hasAttachment: true, rawMediaProto: encodeProto() }
  if (m.documentMessage) return { type: "document", body: m.documentMessage.fileName ?? "", hasAttachment: true, rawMediaProto: encodeProto() }
  if (m.stickerMessage)  return { type: "sticker",  body: "",                              hasAttachment: true, rawMediaProto: encodeProto() }
  return { type: "text", body: "", hasAttachment: false, rawMediaProto: null }
}

const MIME_FROM_TYPE: Record<MessageType, string> = {
  text: "", image: "image/jpeg", video: "video/mp4",
  audio: "audio/ogg", document: "application/octet-stream", sticker: "image/webp",
}
function mimeFromType(type: MessageType): string { return MIME_FROM_TYPE[type] }
