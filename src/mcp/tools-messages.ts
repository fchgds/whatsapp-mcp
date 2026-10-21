import { z } from "zod"
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js"
import type { Database } from "bun:sqlite"
import { listChats } from "../store/chats"
import { getMessages, getMessage, searchMessages } from "../store/messages"
import type { Message } from "../types"

function toISO(ts: number): string { return new Date(ts * 1000).toISOString() }

function msgDTO(m: Message) {
  return {
    id: m.id, chat_jid: m.chatJID, sender_jid: m.senderJID,
    timestamp: toISO(m.timestamp), type: m.type, body: m.body,
    has_attachment: m.hasAttachment, is_downloaded: m.isDownloaded,
  }
}

export function registerMessageTools(server: McpServer, db: Database): void {
  server.tool(
    "list_chats",
    "Lista todos los chats de WhatsApp (individuales y grupos) ordenados por actividad reciente",
    {},
    async () => {
      const chats = listChats(db).map((c) => ({
        jid: c.jid, name: c.name,
        last_message_timestamp: toISO(c.lastMessageTimestamp),
        unread_count: c.unreadCount,
      }))
      return { content: [{ type: "text" as const, text: JSON.stringify(chats) }] }
    }
  )

  server.tool(
    "get_messages",
    "Devuelve mensajes de un chat con paginación y filtro por fecha",
    {
      chat_jid: z.string().describe("JID del chat"),
      limit: z.number().optional().describe("Máximo de mensajes (default 100)"),
      offset: z.number().optional().describe("Offset para paginación (default 0)"),
      from_timestamp: z.number().optional().describe("Filtrar desde Unix timestamp"),
      to_timestamp: z.number().optional().describe("Filtrar hasta Unix timestamp"),
    },
    async ({ chat_jid, limit, offset, from_timestamp, to_timestamp }) => {
      const msgs = getMessages(db, { chatJID: chat_jid, limit, offset, fromTimestamp: from_timestamp, toTimestamp: to_timestamp })
      return { content: [{ type: "text" as const, text: JSON.stringify(msgs.map(msgDTO)) }] }
    }
  )

  server.tool(
    "get_message",
    "Obtiene un mensaje específico por ID",
    { message_id: z.string().describe("ID del mensaje") },
    async ({ message_id }) => {
      const msg = getMessage(db, message_id)
      const text = msg ? JSON.stringify(msgDTO(msg)) : JSON.stringify({ error: "message not found" })
      return { content: [{ type: "text" as const, text }] }
    }
  )

  server.tool(
    "search_messages",
    "Busca mensajes por texto en todos los chats o en uno específico",
    {
      query: z.string().describe("Texto a buscar"),
      chat_jid: z.string().optional().describe("JID del chat para limitar la búsqueda"),
    },
    async ({ query, chat_jid }) => {
      const msgs = searchMessages(db, query, chat_jid)
      return { content: [{ type: "text" as const, text: JSON.stringify(msgs.map(msgDTO)) }] }
    }
  )
}
