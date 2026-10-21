import { z } from "zod"
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js"
import { getStatus, connectQR, connectPhone } from "../whatsapp/client"

export function registerConnectionTools(server: McpServer): void {
  server.tool(
    "get_connection_status",
    "Devuelve el estado actual de la conexión con WhatsApp",
    {},
    async () => ({ content: [{ type: "text" as const, text: JSON.stringify({ status: getStatus() }) }] })
  )

  server.tool(
    "connect_qr",
    "Inicia conexión con WhatsApp vía QR. Devuelve el QR en ASCII para escanearlo con el teléfono.",
    {},
    async () => {
      const qr = await connectQR()
      return { content: [{ type: "text" as const, text: `Escaneá este QR con WhatsApp:\n\n${qr}` }] }
    }
  )

  server.tool(
    "connect_phone",
    "Inicia conexión vía número de teléfono. Devuelve código de 8 caracteres para ingresar en la app.",
    { phone_number: z.string().describe("Número internacional sin +, ej: 5491112345678") },
    async ({ phone_number }) => {
      const code = await connectPhone(phone_number)
      return {
        content: [{
          type: "text" as const,
          text: `Código: ${code}\nIngresalo en WhatsApp > Dispositivos vinculados > Vincular con número de teléfono`,
        }],
      }
    }
  )
}
