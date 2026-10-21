import { loadConfig } from "./config"
import { openDB } from "./store/db"
import { initClient } from "./whatsapp/client"
import { registerHandlers } from "./whatsapp/handlers"
import { startMCP } from "./mcp/server"
import { join } from "node:path"

async function main(): Promise<void> {
  const config = loadConfig()
  const db = openDB(join(config.dbPath, "messages.db"))
  await initClient(config.dbPath, config.logLevel)
  registerHandlers(db)
  await startMCP(db, config.mediaDir)
}

main().catch((err: Error) => {
  process.stderr.write(`Fatal: ${err.message}\n`)
  process.exit(1)
})
