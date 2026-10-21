import { proto, downloadMediaMessage } from "@whiskeysockets/baileys"
import { waState } from "./client"

export async function downloadMedia(rawProto: Buffer, type: string): Promise<Buffer> {
  if (!waState.socket) throw new Error("No conectado a WhatsApp")
  if (waState.status !== "connected") throw new Error(`Estado actual: ${waState.status}`)

  const message = proto.Message.decode(rawProto)

  const fakeMsg = {
    key: { remoteJid: "status@broadcast", id: "dl" },
    message,
    messageTimestamp: Math.floor(Date.now() / 1000),
  }

  const result = await downloadMediaMessage(
    fakeMsg as Parameters<typeof downloadMediaMessage>[0],
    "buffer",
    {},
    { logger: undefined as never, reuploadRequest: waState.socket.updateMediaMessage }
  )

  if (!(result instanceof Buffer)) throw new Error("La descarga no devolvió un Buffer")
  return result
}
