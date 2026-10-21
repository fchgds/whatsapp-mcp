import { z } from "zod"
import { createHash } from "node:crypto"
import { mkdirSync, writeFileSync } from "node:fs"
import { join } from "node:path"
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js"
import type { Database } from "bun:sqlite"
import { getMessage } from "../store/messages"
import { getAttachment, upsertAttachment, markDownloaded } from "../store/attachments"
import { downloadMedia } from "../whatsapp/media"

const MIME_TO_EXT: Record<string, string> = {
  "image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp",
  "video/mp4": ".mp4", "audio/ogg": ".ogg", "audio/mp4": ".m4a",
  "application/pdf": ".pdf", "application/octet-stream": "",
}
const TYPE_TO_MIME: Record<string, string> = {
  image: "image/jpeg", video: "video/mp4", audio: "audio/ogg",
  document: "application/octet-stream", sticker: "image/webp",
}

function sanitizeJID(jid: string): string { return jid.replace(/[@.+]/g, "_") }

export function registerAttachmentTools(server: McpServer, db: Database, mediaDir: string): void {
  server.tool(
    "download_attachment",
    "Descarga el adjunto de un mensaje a la carpeta local y devuelve la ruta en disco",
    { message_id: z.string().describe("ID del mensaje con adjunto") },
    async ({ message_id }) => {
      const msg = getMessage(db, message_id)
      if (!msg) throw new Error(`Mensaje ${message_id} no encontrado`)
      if (!msg.hasAttachment) throw new Error(`El mensaje ${message_id} no tiene adjunto`)

      const existing = getAttachment(db, message_id)
      if (existing?.filePath) {
        return { content: [{ type: "text" as const, text: JSON.stringify({ file_path: existing.filePath, mime_type: existing.mimeType }) }] }
      }

      if (!msg.rawMediaProto) throw new Error("No hay datos de media para este mensaje")

      const data = await downloadMedia(msg.rawMediaProto, msg.type)
      const mime = existing?.mimeType || TYPE_TO_MIME[msg.type] || "application/octet-stream"
      const ext = MIME_TO_EXT[mime] ?? ""
      // mediaDir viene del parámetro
      const dir = join(mediaDir, sanitizeJID(msg.chatJID))
      mkdirSync(dir, { recursive: true })
      const filePath = join(dir, `${message_id}${ext}`)
      writeFileSync(filePath, data)

      const sha256 = createHash("sha256").update(data).digest("hex")
      upsertAttachment(db, { messageID: message_id, filePath, mimeType: mime, fileSize: data.length, sha256 })
      markDownloaded(db, message_id, filePath)

      return {
        content: [{
          type: "text" as const,
          text: JSON.stringify({ file_path: filePath, mime_type: mime, file_size: data.length, sha256 }),
        }],
      }
    }
  )
}
