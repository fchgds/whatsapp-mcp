import { z } from "zod"
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js"
import type { Database } from "bun:sqlite"
import { listContacts, searchContacts } from "../store/contacts"

export function registerContactTools(server: McpServer, db: Database): void {
  server.tool(
    "list_contacts",
    "Lista contactos de WhatsApp sincronizados",
    {
      limit: z.number().optional().describe("Máximo de contactos (default 200)"),
    },
    async ({ limit }) => ({
      content: [{ type: "text" as const, text: JSON.stringify(listContacts(db, limit)) }],
    })
  )

  server.tool(
    "search_contacts",
    "Busca contactos por nombre, notify, nombre verificado, estado, JID o LID",
    {
      query: z.string().describe("Texto a buscar"),
      limit: z.number().optional().describe("Máximo de contactos (default 50)"),
    },
    async ({ query, limit }) => ({
      content: [{ type: "text" as const, text: JSON.stringify(searchContacts(db, query, limit)) }],
    })
  )
}
