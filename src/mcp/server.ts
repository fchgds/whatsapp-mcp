import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js"
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js"
import type { Database } from "bun:sqlite"
import { registerConnectionTools } from "./tools-connection"
import { registerMessageTools } from "./tools-messages"
import { registerAttachmentTools } from "./tools-attachments"
import { registerContactTools } from "./tools-contacts"

export async function startMCP(db: Database, mediaDir: string): Promise<void> {
  const server = new McpServer({ name: "whatsapp-mcp", version: "1.0.0" })
  registerConnectionTools(server)
  registerMessageTools(server, db)
  registerAttachmentTools(server, db, mediaDir)
  registerContactTools(server, db)
  await server.connect(new StdioServerTransport())
}
