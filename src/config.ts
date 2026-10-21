export interface Config {
  mediaDir: string
  dbPath: string
  logLevel: string
}

export function loadConfig(): Config {
  const mediaDir = process.env.WHATSAPP_MEDIA_DIR
  if (!mediaDir) throw new Error("WHATSAPP_MEDIA_DIR environment variable is required")
  return {
    mediaDir,
    dbPath: process.env.WHATSAPP_DB_PATH ?? "./data",
    logLevel: process.env.LOG_LEVEL ?? "info",
  }
}
