# WhatsApp MCP TypeScript Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Construir un servidor MCP en TypeScript/Bun que conecta a WhatsApp Web (Baileys) y expone herramientas de solo lectura al asistente de IA para leer mensajes y descargar adjuntos.

**Architecture:** Tres capas funcionales (store, whatsapp, mcp) comunicadas por inyección de dependencias. `bun:sqlite` para persistencia sincrónica. Baileys para el protocolo WhatsApp multi-device. `@modelcontextprotocol/sdk` con transporte stdio.

**Tech Stack:** Bun 1.x, TypeScript strict, `@whiskeysockets/baileys`, `@modelcontextprotocol/sdk`, `bun:sqlite` (built-in), `qrcode-terminal`, `pino`, `zod`

## Global Constraints

- Runtime: Bun 1.x — todos los comandos usan `bun`, nunca `node` o `npx`
- TypeScript strict mode en tsconfig
- Módulos funcionales — sin clases exportadas
- `bun:sqlite` sincrónico en toda la capa store — sin Promises en el store
- Logger pino en nivel `"silent"` — ningún log va a stdout (interfiere con stdio MCP)
- Archivos de auth: `<WHATSAPP_DB_PATH>/auth/`
- Base de datos: `<WHATSAPP_DB_PATH>/messages.db`
- Variable `WHATSAPP_MEDIA_DIR` es requerida; falla al arrancar si falta

---

## Mapa de archivos

| Archivo | Responsabilidad |
|---------|----------------|
| `package.json` | Módulo Bun, scripts, dependencias |
| `tsconfig.json` | Config TypeScript strict |
| `.env.example` | Plantilla de variables de entorno |
| `.gitignore` | Excluir data/, .env, dist/ |
| `src/types.ts` | Interfaces: Chat, Message, Attachment, GetMessagesOpts, MessageType |
| `src/config.ts` | Lee y valida variables de entorno — exporta `loadConfig(): Config` |
| `src/store/db.ts` | Abre bun:sqlite, aplica schema — exporta `openDB`, `applySchema` |
| `src/store/chats.ts` | `upsertChat`, `listChats` |
| `src/store/chats.test.ts` | Tests de chats store con DB en memoria |
| `src/store/messages.ts` | `insertMessage`, `getMessages`, `getMessage`, `searchMessages` |
| `src/store/messages.test.ts` | Tests de messages store con DB en memoria |
| `src/store/attachments.ts` | `upsertAttachment`, `getAttachment`, `markDownloaded` |
| `src/whatsapp/client.ts` | `waState`, `initClient`, `connectQR`, `connectPhone`, `getStatus` |
| `src/whatsapp/handlers.ts` | `registerHandlers(db)` — eventos Baileys → store |
| `src/whatsapp/media.ts` | `downloadMedia(rawProto, type)` — descarga bytes de adjunto |
| `src/mcp/server.ts` | `startMCP(db)` — crea McpServer, registra tools, conecta stdio |
| `src/mcp/tools-connection.ts` | `registerConnectionTools(server)` |
| `src/mcp/tools-messages.ts` | `registerMessageTools(server, db)` |
| `src/mcp/tools-attachments.ts` | `registerAttachmentTools(server, db)` |
| `src/index.ts` | Entry point: orquesta arranque de todas las capas |

---

## Task 1: Scaffold del proyecto

**Files:**
- Create: `package.json`
- Create: `tsconfig.json`
- Create: `.env.example`
- Create: `.gitignore`

**Interfaces:**
- Consumes: nada
- Produces: estructura base del proyecto, dependencias instaladas

- [ ] **Step 1.1: Crear package.json**

```json
{
  "name": "whatsapp-mcp",
  "version": "1.0.0",
  "private": true,
  "scripts": {
    "start": "bun run src/index.ts",
    "test": "bun test"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "^1.12.1",
    "@whiskeysockets/baileys": "^6.7.18",
    "pino": "^9.6.0",
    "qrcode-terminal": "^0.12.0"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "@types/qrcode-terminal": "^0.12.2",
    "typescript": "^5.8.0"
  }
}
```

- [ ] **Step 1.2: Crear tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "skipLibCheck": true,
    "esModuleInterop": true
  },
  "include": ["src/**/*"]
}
```

- [ ] **Step 1.3: Crear .env.example**

```
WHATSAPP_MEDIA_DIR=C:/whatsapp-media
WHATSAPP_DB_PATH=./data
LOG_LEVEL=info
```

- [ ] **Step 1.4: Crear .gitignore**

```
node_modules/
dist/
data/
.env
*.db
```

- [ ] **Step 1.5: Crear estructura de directorios e instalar dependencias**

```bash
mkdir -p src/store src/whatsapp src/mcp
bun install
```

Resultado esperado: se crea `bun.lockb` y `node_modules/`.

- [ ] **Step 1.6: Commit**

```bash
git add package.json tsconfig.json .env.example .gitignore bun.lockb
git commit -m "feat: scaffold TypeScript/Bun project"
```

---

## Task 2: Tipos y configuración

**Files:**
- Create: `src/types.ts`
- Create: `src/config.ts`

**Interfaces:**
- Consumes: variables de entorno `WHATSAPP_MEDIA_DIR`, `WHATSAPP_DB_PATH`, `LOG_LEVEL`
- Produces:
  - Tipos: `Chat`, `Message`, `Attachment`, `GetMessagesOpts`, `MessageType`
  - `loadConfig(): Config` — lanza Error si `WHATSAPP_MEDIA_DIR` no está definida
  - `Config` interface: `{ mediaDir: string; dbPath: string; logLevel: string }`

- [ ] **Step 2.1: Crear src/types.ts**

```typescript
export type MessageType = "text" | "image" | "video" | "audio" | "document" | "sticker"

export interface Chat {
  jid: string
  name: string
  lastMessageTimestamp: number // Unix epoch seconds
  unreadCount: number
}

export interface Message {
  id: string
  chatJID: string
  senderJID: string
  timestamp: number // Unix epoch seconds
  type: MessageType
  body: string
  hasAttachment: boolean
  isDownloaded: boolean
  rawMediaProto: Buffer | null
}

export interface Attachment {
  messageID: string
  filePath: string
  mimeType: string
  fileSize: number
  sha256: string
}

export interface GetMessagesOpts {
  chatJID: string
  limit?: number
  offset?: number
  fromTimestamp?: number
  toTimestamp?: number
}
```

- [ ] **Step 2.2: Crear src/config.ts**

```typescript
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
```

- [ ] **Step 2.3: Verificar que compila**

```bash
bun build src/config.ts --target bun 2>&1 | head -5
```

Resultado esperado: sin errores.

- [ ] **Step 2.4: Commit**

```bash
git add src/types.ts src/config.ts
git commit -m "feat: types and config module"
```

---

## Task 3: Store — inicialización de DB

**Files:**
- Create: `src/store/db.ts`

**Interfaces:**
- Consumes: `bun:sqlite`
- Produces:
  - `openDB(filePath: string): Database` — crea dirs, abre DB, aplica schema
  - `applySchema(db: Database): void` — aplica schema a DB existente (usado en tests)

- [ ] **Step 3.1: Crear src/store/db.ts**

```typescript
import { Database } from "bun:sqlite"
import { mkdirSync } from "node:fs"
import { dirname } from "node:path"

const SCHEMA = `
CREATE TABLE IF NOT EXISTS chats (
  jid                    TEXT PRIMARY KEY,
  name                   TEXT NOT NULL DEFAULT '',
  last_message_timestamp INTEGER NOT NULL DEFAULT 0,
  unread_count           INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS messages (
  id               TEXT PRIMARY KEY,
  chat_jid         TEXT NOT NULL,
  sender_jid       TEXT NOT NULL,
  timestamp        INTEGER NOT NULL,
  type             TEXT NOT NULL,
  body             TEXT NOT NULL DEFAULT '',
  has_attachment   INTEGER NOT NULL DEFAULT 0,
  is_downloaded    INTEGER NOT NULL DEFAULT 0,
  raw_media_proto  BLOB,
  FOREIGN KEY (chat_jid) REFERENCES chats(jid)
);

CREATE INDEX IF NOT EXISTS idx_messages_chat_ts ON messages(chat_jid, timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_body    ON messages(body);

CREATE TABLE IF NOT EXISTS attachments (
  message_id TEXT PRIMARY KEY,
  file_path  TEXT NOT NULL DEFAULT '',
  mime_type  TEXT NOT NULL DEFAULT '',
  file_size  INTEGER NOT NULL DEFAULT 0,
  sha256     TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (message_id) REFERENCES messages(id)
);
`

export function openDB(filePath: string): Database {
  if (filePath !== ":memory:") {
    mkdirSync(dirname(filePath), { recursive: true })
  }
  const db = new Database(filePath, { create: true })
  db.run("PRAGMA foreign_keys = ON")
  db.run("PRAGMA journal_mode = WAL")
  applySchema(db)
  return db
}

export function applySchema(db: Database): void {
  db.run(SCHEMA)
}
```

- [ ] **Step 3.2: Verificar que compila**

```bash
bun build src/store/db.ts --target bun 2>&1 | head -5
```

Resultado esperado: sin errores.

- [ ] **Step 3.3: Commit**

```bash
git add src/store/db.ts
git commit -m "feat: SQLite database initialization and schema"
```

---

## Task 4: Store — chats

**Files:**
- Create: `src/store/chats.ts`
- Create: `src/store/chats.test.ts`

**Interfaces:**
- Consumes: `Database` de `bun:sqlite`, `Chat` de `src/types.ts`, `applySchema` de `src/store/db.ts`
- Produces:
  - `upsertChat(db: Database, chat: Chat): void`
  - `listChats(db: Database): Chat[]`

- [ ] **Step 4.1: Escribir src/store/chats.test.ts primero**

```typescript
import { describe, it, expect, beforeEach } from "bun:test"
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
```

- [ ] **Step 4.2: Ejecutar test — verificar que falla**

```bash
bun test src/store/chats.test.ts 2>&1 | head -20
```

Resultado esperado: error (chats.ts no existe aún).

- [ ] **Step 4.3: Implementar src/store/chats.ts**

```typescript
import type { Database } from "bun:sqlite"
import type { Chat } from "../types"

type ChatRow = {
  jid: string
  name: string
  last_message_timestamp: number
  unread_count: number
}

export function upsertChat(db: Database, chat: Chat): void {
  db.run(
    `INSERT INTO chats (jid, name, last_message_timestamp, unread_count)
     VALUES (?, ?, ?, ?)
     ON CONFLICT(jid) DO UPDATE SET
       name = excluded.name,
       last_message_timestamp = excluded.last_message_timestamp,
       unread_count = excluded.unread_count`,
    [chat.jid, chat.name, chat.lastMessageTimestamp, chat.unreadCount]
  )
}

export function listChats(db: Database): Chat[] {
  const rows = db.query(
    `SELECT jid, name, last_message_timestamp, unread_count
     FROM chats ORDER BY last_message_timestamp DESC`
  ).all() as ChatRow[]
  return rows.map((r) => ({
    jid: r.jid,
    name: r.name,
    lastMessageTimestamp: r.last_message_timestamp,
    unreadCount: r.unread_count,
  }))
}
```

- [ ] **Step 4.4: Ejecutar tests — verificar que pasan**

```bash
bun test src/store/chats.test.ts
```

Resultado esperado: 3 tests en PASS.

- [ ] **Step 4.5: Commit**

```bash
git add src/store/chats.ts src/store/chats.test.ts
git commit -m "feat: chat store with upsert and list"
```

---

## Task 5: Store — messages

**Files:**
- Create: `src/store/messages.ts`
- Create: `src/store/messages.test.ts`

**Interfaces:**
- Consumes: `Database`, `Message`, `GetMessagesOpts`, `upsertChat`
- Produces:
  - `insertMessage(db: Database, msg: Message): void`
  - `getMessages(db: Database, opts: GetMessagesOpts): Message[]`
  - `getMessage(db: Database, id: string): Message | null`
  - `searchMessages(db: Database, query: string, chatJID?: string): Message[]`

- [ ] **Step 5.1: Escribir src/store/messages.test.ts primero**

```typescript
import { describe, it, expect } from "bun:test"
import { Database } from "bun:sqlite"
import { applySchema } from "./db"
import { upsertChat } from "./chats"
import { insertMessage, getMessages, getMessage, searchMessages } from "./messages"
import type { Message } from "../types"

const CHAT = "5491112345678@s.whatsapp.net"

function makeTestDB(): Database {
  const db = new Database(":memory:")
  db.run("PRAGMA foreign_keys = ON")
  applySchema(db)
  upsertChat(db, { jid: CHAT, name: "Test", lastMessageTimestamp: 0, unreadCount: 0 })
  return db
}

function makeMsg(overrides: Partial<Message> = {}): Message {
  return {
    id: "msg1", chatJID: CHAT, senderJID: CHAT, timestamp: 1700000000,
    type: "text", body: "Hola", hasAttachment: false, isDownloaded: false, rawMediaProto: null,
    ...overrides,
  }
}

describe("messages store", () => {
  it("inserts and retrieves a message by id", () => {
    const db = makeTestDB()
    insertMessage(db, makeMsg({ body: "Hola mundo" }))
    const msg = getMessage(db, "msg1")
    expect(msg).not.toBeNull()
    expect(msg!.body).toBe("Hola mundo")
    expect(msg!.hasAttachment).toBe(false)
  })

  it("returns null for unknown id", () => {
    const db = makeTestDB()
    expect(getMessage(db, "nope")).toBeNull()
  })

  it("paginates with limit and offset", () => {
    const db = makeTestDB()
    for (let i = 0; i < 5; i++) {
      insertMessage(db, makeMsg({ id: `m${i}`, timestamp: 1700000000 + i, body: `msg ${i}` }))
    }
    const page = getMessages(db, { chatJID: CHAT, limit: 2, offset: 1 })
    expect(page).toHaveLength(2)
    expect(page[0].id).toBe("m1")
    expect(page[1].id).toBe("m2")
  })

  it("filters by fromTimestamp and toTimestamp", () => {
    const db = makeTestDB()
    insertMessage(db, makeMsg({ id: "a", timestamp: 100 }))
    insertMessage(db, makeMsg({ id: "b", timestamp: 200 }))
    insertMessage(db, makeMsg({ id: "c", timestamp: 300 }))
    const msgs = getMessages(db, { chatJID: CHAT, fromTimestamp: 150, toTimestamp: 250 })
    expect(msgs).toHaveLength(1)
    expect(msgs[0].id).toBe("b")
  })

  it("searches messages by body text", () => {
    const db = makeTestDB()
    insertMessage(db, makeMsg({ id: "a", body: "reunión mañana" }))
    insertMessage(db, makeMsg({ id: "b", body: "hasta luego" }))
    const results = searchMessages(db, "reunión")
    expect(results).toHaveLength(1)
    expect(results[0].id).toBe("a")
  })

  it("searches within a specific chat", () => {
    const db = makeTestDB()
    const OTHER = "other@s.whatsapp.net"
    upsertChat(db, { jid: OTHER, name: "Other", lastMessageTimestamp: 0, unreadCount: 0 })
    insertMessage(db, makeMsg({ id: "a", body: "hello" }))
    insertMessage(db, makeMsg({ id: "b", chatJID: OTHER, senderJID: OTHER, body: "hello" }))
    const results = searchMessages(db, "hello", CHAT)
    expect(results).toHaveLength(1)
    expect(results[0].chatJID).toBe(CHAT)
  })
})
```

- [ ] **Step 5.2: Ejecutar test — verificar que falla**

```bash
bun test src/store/messages.test.ts 2>&1 | head -20
```

Resultado esperado: error de compilación.

- [ ] **Step 5.3: Implementar src/store/messages.ts**

```typescript
import type { Database } from "bun:sqlite"
import type { Message, GetMessagesOpts } from "../types"

type MessageRow = {
  id: string; chat_jid: string; sender_jid: string; timestamp: number
  type: string; body: string; has_attachment: number; is_downloaded: number
  raw_media_proto: Buffer | null
}

function rowToMessage(r: MessageRow): Message {
  return {
    id: r.id, chatJID: r.chat_jid, senderJID: r.sender_jid, timestamp: r.timestamp,
    type: r.type as Message["type"], body: r.body,
    hasAttachment: r.has_attachment === 1, isDownloaded: r.is_downloaded === 1,
    rawMediaProto: r.raw_media_proto,
  }
}

const SELECT = `SELECT id, chat_jid, sender_jid, timestamp, type, body, has_attachment, is_downloaded, raw_media_proto FROM messages`

export function insertMessage(db: Database, msg: Message): void {
  db.run(
    `INSERT OR IGNORE INTO messages (id, chat_jid, sender_jid, timestamp, type, body, has_attachment, is_downloaded, raw_media_proto)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [msg.id, msg.chatJID, msg.senderJID, msg.timestamp, msg.type, msg.body,
     msg.hasAttachment ? 1 : 0, msg.isDownloaded ? 1 : 0, msg.rawMediaProto ?? null]
  )
}

export function getMessage(db: Database, id: string): Message | null {
  const row = db.query(`${SELECT} WHERE id = ?`).get(id) as MessageRow | null
  return row ? rowToMessage(row) : null
}

export function getMessages(db: Database, opts: GetMessagesOpts): Message[] {
  const { chatJID, limit = 100, offset = 0, fromTimestamp, toTimestamp } = opts
  let sql = `${SELECT} WHERE chat_jid = ?`
  const params: (string | number)[] = [chatJID]
  if (fromTimestamp !== undefined) { sql += " AND timestamp >= ?"; params.push(fromTimestamp) }
  if (toTimestamp !== undefined)   { sql += " AND timestamp <= ?"; params.push(toTimestamp) }
  sql += " ORDER BY timestamp ASC LIMIT ? OFFSET ?"
  params.push(limit, offset)
  return (db.query(sql).all(...params) as MessageRow[]).map(rowToMessage)
}

export function searchMessages(db: Database, query: string, chatJID?: string): Message[] {
  let sql = `${SELECT} WHERE body LIKE ?`
  const params: (string | number)[] = [`%${query}%`]
  if (chatJID) { sql += " AND chat_jid = ?"; params.push(chatJID) }
  sql += " ORDER BY timestamp DESC LIMIT 200"
  return (db.query(sql).all(...params) as MessageRow[]).map(rowToMessage)
}
```

- [ ] **Step 5.4: Ejecutar tests — verificar que pasan**

```bash
bun test src/store/messages.test.ts
```

Resultado esperado: 6 tests en PASS.

- [ ] **Step 5.5: Commit**

```bash
git add src/store/messages.ts src/store/messages.test.ts
git commit -m "feat: message store with insert, get, pagination and search"
```

---

## Task 6: Store — attachments

**Files:**
- Create: `src/store/attachments.ts`

**Interfaces:**
- Consumes: `Database`, `Attachment` de `src/types.ts`
- Produces:
  - `upsertAttachment(db: Database, att: Attachment): void`
  - `getAttachment(db: Database, messageID: string): Attachment | null`
  - `markDownloaded(db: Database, messageID: string, filePath: string): void`

- [ ] **Step 6.1: Implementar src/store/attachments.ts**

```typescript
import type { Database } from "bun:sqlite"
import type { Attachment } from "../types"

type AttachmentRow = {
  message_id: string; file_path: string; mime_type: string; file_size: number; sha256: string
}

function rowToAttachment(r: AttachmentRow): Attachment {
  return { messageID: r.message_id, filePath: r.file_path, mimeType: r.mime_type, fileSize: r.file_size, sha256: r.sha256 }
}

export function upsertAttachment(db: Database, att: Attachment): void {
  db.run(
    `INSERT INTO attachments (message_id, file_path, mime_type, file_size, sha256) VALUES (?, ?, ?, ?, ?)
     ON CONFLICT(message_id) DO UPDATE SET file_path=excluded.file_path, mime_type=excluded.mime_type, file_size=excluded.file_size, sha256=excluded.sha256`,
    [att.messageID, att.filePath, att.mimeType, att.fileSize, att.sha256]
  )
}

export function getAttachment(db: Database, messageID: string): Attachment | null {
  const row = db.query(
    `SELECT message_id, file_path, mime_type, file_size, sha256 FROM attachments WHERE message_id = ?`
  ).get(messageID) as AttachmentRow | null
  return row ? rowToAttachment(row) : null
}

export function markDownloaded(db: Database, messageID: string, filePath: string): void {
  db.run(`UPDATE messages SET is_downloaded = 1 WHERE id = ?`, [messageID])
  db.run(`UPDATE attachments SET file_path = ? WHERE message_id = ?`, [filePath, messageID])
}
```

- [ ] **Step 6.2: Ejecutar todos los tests del store**

```bash
bun test src/store/
```

Resultado esperado: todos los tests previos en PASS (attachments.ts compila correctamente).

- [ ] **Step 6.3: Commit**

```bash
git add src/store/attachments.ts
git commit -m "feat: attachment store"
```

---

## Task 7: WhatsApp client

**Files:**
- Create: `src/whatsapp/client.ts`

**Interfaces:**
- Consumes: `makeWASocket`, `useMultiFileAuthState`, `DisconnectReason` de `@whiskeysockets/baileys`; `pino`; `qrcode-terminal`
- Produces:
  - `Status` tipo: `"disconnected" | "connecting" | "waiting_qr" | "waiting_phone" | "connected"`
  - `waState: { status: Status; socket: WASocket | null }` (exportado mutable)
  - `initClient(dbPath: string): Promise<void>`
  - `connectQR(): Promise<string>` — retorna QR en ASCII, espera si aún no llegó
  - `connectPhone(phone: string): Promise<string>` — retorna código de 8 caracteres
  - `getStatus(): Status`

- [ ] **Step 7.1: Implementar src/whatsapp/client.ts**

```typescript
import makeWASocket, {
  DisconnectReason,
  useMultiFileAuthState,
  type WASocket,
  type ConnectionState,
} from "@whiskeysockets/baileys"
import pino from "pino"
import { join } from "node:path"
import { mkdirSync } from "node:fs"
import qrcodeTerminal from "qrcode-terminal"

export type Status = "disconnected" | "connecting" | "waiting_qr" | "waiting_phone" | "connected"

export const waState: { status: Status; socket: WASocket | null } = {
  status: "disconnected",
  socket: null,
}

let authDir = ""
let saveCreds: (() => Promise<void>) | null = null
let pendingQR: string | undefined
let qrResolve: ((ascii: string) => void) | undefined

const logger = pino({ level: "silent" })

export async function initClient(dbPath: string): Promise<void> {
  authDir = join(dbPath, "auth")
  mkdirSync(authDir, { recursive: true })
  await createSocket()
}

async function createSocket(): Promise<void> {
  const { state, saveCreds: sc } = await useMultiFileAuthState(authDir)
  saveCreds = sc
  const sock = makeWASocket({ auth: state, printQRInTerminal: false, logger })
  waState.socket = sock
  waState.status = "connecting"
  sock.ev.on("creds.update", () => saveCreds?.())
  sock.ev.on("connection.update", handleConnectionUpdate)
}

function handleConnectionUpdate(update: Partial<ConnectionState>): void {
  const { connection, lastDisconnect, qr } = update
  if (qr) {
    waState.status = "waiting_qr"
    pendingQR = qr
    const ascii = qrToASCII(qr)
    qrResolve?.(ascii)
    qrResolve = undefined
  }
  if (connection === "open") {
    waState.status = "connected"
    pendingQR = undefined
  }
  if (connection === "close") {
    waState.status = "disconnected"
    const code = (lastDisconnect?.error as { output?: { statusCode?: number } })?.output?.statusCode
    if (code !== DisconnectReason.loggedOut) createSocket()
  }
}

export async function connectQR(): Promise<string> {
  if (waState.status === "connected") throw new Error("Ya conectado")
  if (pendingQR) return qrToASCII(pendingQR)
  return new Promise<string>((resolve) => { qrResolve = resolve })
}

export async function connectPhone(phone: string): Promise<string> {
  if (!waState.socket) throw new Error("Cliente no inicializado")
  if (waState.status === "connected") throw new Error("Ya conectado")
  waState.status = "waiting_phone"
  return waState.socket.requestPairingCode(phone)
}

export function getStatus(): Status {
  return waState.status
}

function qrToASCII(code: string): string {
  let result = ""
  qrcodeTerminal.generate(code, { small: true }, (qr: string) => { result = qr })
  return result
}
```

- [ ] **Step 7.2: Verificar que compila**

```bash
bun build src/whatsapp/client.ts --target bun 2>&1 | head -10
```

Resultado esperado: sin errores.

- [ ] **Step 7.3: Commit**

```bash
git add src/whatsapp/client.ts
git commit -m "feat: WhatsApp client with QR, phone linking and auto-reconnect"
```

---

## Task 8: WhatsApp event handlers

**Files:**
- Create: `src/whatsapp/handlers.ts`

**Interfaces:**
- Consumes: `waState` de `src/whatsapp/client.ts`; `proto`, `type WAMessage` de `@whiskeysockets/baileys`; `Database`; `upsertChat`; `insertMessage`; `upsertAttachment`
- Produces:
  - `registerHandlers(db: Database): void`

- [ ] **Step 8.1: Implementar src/whatsapp/handlers.ts**

```typescript
import { proto, type WAMessage } from "@whiskeysockets/baileys"
import type { Database } from "bun:sqlite"
import { waState } from "./client"
import { upsertChat } from "../store/chats"
import { insertMessage } from "../store/messages"
import { upsertAttachment } from "../store/attachments"
import type { Message, MessageType } from "../types"

export function registerHandlers(db: Database): void {
  const sock = waState.socket
  if (!sock) throw new Error("Socket no inicializado — llamá initClient primero")

  sock.ev.on("messages.upsert", ({ messages }) => {
    for (const msg of messages) {
      if (msg.message) handleMessage(db, msg)
    }
  })

  sock.ev.on("messaging-history.set", ({ chats, messages }) => {
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
}

function handleMessage(db: Database, waMsg: WAMessage): void {
  const key = waMsg.key
  if (!key?.remoteJid || !waMsg.message) return

  const chatJID = key.remoteJid
  const senderJID = key.participant ?? key.remoteJid
  const timestamp = Number(waMsg.messageTimestamp ?? 0)

  upsertChat(db, { jid: chatJID, name: waMsg.pushName ?? chatJID, lastMessageTimestamp: timestamp, unreadCount: 0 })

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

function extractContent(waMsg: WAMessage): { type: MessageType; body: string; hasAttachment: boolean; rawMediaProto: Buffer | null } {
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
```

- [ ] **Step 8.2: Verificar que compila**

```bash
bun build src/whatsapp/handlers.ts --target bun 2>&1 | head -10
```

Resultado esperado: sin errores.

- [ ] **Step 8.3: Commit**

```bash
git add src/whatsapp/handlers.ts
git commit -m "feat: WhatsApp event handlers for messages and history sync"
```

---

## Task 9: WhatsApp media download

**Files:**
- Create: `src/whatsapp/media.ts`

**Interfaces:**
- Consumes: `waState` de `src/whatsapp/client.ts`; `proto`, `downloadMediaMessage` de `@whiskeysockets/baileys`
- Produces:
  - `downloadMedia(rawProto: Buffer, type: string): Promise<Buffer>`

- [ ] **Step 9.1: Implementar src/whatsapp/media.ts**

```typescript
import { proto, downloadMediaMessage } from "@whiskeysockets/baileys"
import { waState } from "./client"

export async function downloadMedia(rawProto: Buffer, type: string): Promise<Buffer> {
  if (!waState.socket) throw new Error("No conectado a WhatsApp")
  if (waState.status !== "connected") throw new Error(`Estado actual: ${waState.status}`)

  const message = proto.Message.decode(rawProto)

  // downloadMediaMessage espera un objeto WAMessage-like
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
```

- [ ] **Step 9.2: Verificar que compila**

```bash
bun build src/whatsapp/media.ts --target bun 2>&1 | head -10
```

Resultado esperado: sin errores.

- [ ] **Step 9.3: Commit**

```bash
git add src/whatsapp/media.ts
git commit -m "feat: WhatsApp media download module"
```

---

## Task 10: MCP server y connection tools

**Files:**
- Create: `src/mcp/tools-connection.ts`
- Create: `src/mcp/server.ts`

**Interfaces:**
- Consumes: `McpServer` de `@modelcontextprotocol/sdk/server/mcp.js`; `StdioServerTransport` de `@modelcontextprotocol/sdk/server/stdio.js`; `getStatus`, `connectQR`, `connectPhone`; `Database`; `z` de `zod`
- Produces:
  - `registerConnectionTools(server: McpServer): void`
  - `startMCP(db: Database): Promise<void>`

- [ ] **Step 10.1: Implementar src/mcp/tools-connection.ts**

```typescript
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
```

- [ ] **Step 10.2: Implementar src/mcp/server.ts**

```typescript
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js"
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js"
import type { Database } from "bun:sqlite"
import { registerConnectionTools } from "./tools-connection"
import { registerMessageTools } from "./tools-messages"
import { registerAttachmentTools } from "./tools-attachments"

export async function startMCP(db: Database): Promise<void> {
  const server = new McpServer({ name: "whatsapp-mcp", version: "1.0.0" })
  registerConnectionTools(server)
  registerMessageTools(server, db)
  registerAttachmentTools(server, db)
  await server.connect(new StdioServerTransport())
}
```

- [ ] **Step 10.3: Verificar compilación**

```bash
bun build src/mcp/tools-connection.ts --target bun 2>&1 | head -10
```

Resultado esperado: sin errores.

- [ ] **Step 10.4: Commit**

```bash
git add src/mcp/server.ts src/mcp/tools-connection.ts
git commit -m "feat: MCP server and connection tools"
```

---

## Task 11: MCP message tools

**Files:**
- Create: `src/mcp/tools-messages.ts`

**Interfaces:**
- Consumes: `listChats`; `getMessages`, `getMessage`, `searchMessages`; `Database`; `z`; `McpServer`
- Produces:
  - `registerMessageTools(server: McpServer, db: Database): void`

- [ ] **Step 11.1: Implementar src/mcp/tools-messages.ts**

```typescript
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
```

- [ ] **Step 11.2: Verificar compilación**

```bash
bun build src/mcp/tools-messages.ts --target bun 2>&1 | head -10
```

Resultado esperado: sin errores.

- [ ] **Step 11.3: Commit**

```bash
git add src/mcp/tools-messages.ts
git commit -m "feat: MCP tools for listing chats and reading messages"
```

---

## Task 12: MCP attachment tool

**Files:**
- Create: `src/mcp/tools-attachments.ts`

**Interfaces:**
- Consumes: `getMessage`; `getAttachment`, `upsertAttachment`, `markDownloaded`; `downloadMedia`; `Database`; `z`; `McpServer`
- Produces:
  - `registerAttachmentTools(server: McpServer, db: Database): void`

- [ ] **Step 12.1: Implementar src/mcp/tools-attachments.ts**

```typescript
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

export function registerAttachmentTools(server: McpServer, db: Database): void {
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
      const mediaDir = process.env.WHATSAPP_MEDIA_DIR ?? "./media"
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
```

- [ ] **Step 12.2: Verificar compilación**

```bash
bun build src/mcp/tools-attachments.ts --target bun 2>&1 | head -10
```

Resultado esperado: sin errores.

- [ ] **Step 12.3: Commit**

```bash
git add src/mcp/tools-attachments.ts
git commit -m "feat: MCP download_attachment tool"
```

---

## Task 13: Entry point

**Files:**
- Create: `src/index.ts`
- Create: `.env`

**Interfaces:**
- Consumes: todos los módulos anteriores
- Produces: proceso ejecutable que arranca el servidor MCP en stdio

- [ ] **Step 13.1: Implementar src/index.ts**

```typescript
import { loadConfig } from "./config"
import { openDB } from "./store/db"
import { initClient } from "./whatsapp/client"
import { registerHandlers } from "./whatsapp/handlers"
import { startMCP } from "./mcp/server"
import { join } from "node:path"

async function main(): Promise<void> {
  const config = loadConfig()
  const db = openDB(join(config.dbPath, "messages.db"))
  await initClient(config.dbPath)
  registerHandlers(db)
  await startMCP(db)
}

main().catch((err: Error) => {
  process.stderr.write(`Fatal: ${err.message}\n`)
  process.exit(1)
})
```

- [ ] **Step 13.2: Crear .env para prueba local**

Crear el archivo `.env` en la raíz del proyecto:

```
WHATSAPP_MEDIA_DIR=C:/whatsapp-media
WHATSAPP_DB_PATH=./data
LOG_LEVEL=info
```

- [ ] **Step 13.3: Verificar build completo**

```bash
bun build src/index.ts --target bun 2>&1 | head -20
```

Resultado esperado: sin errores.

- [ ] **Step 13.4: Ejecutar todos los tests**

```bash
bun test
```

Resultado esperado: todos los tests en PASS.

- [ ] **Step 13.5: Commit**

```bash
git add src/index.ts .env
git commit -m "feat: main entry point wiring all layers"
```

---

## Task 14: Integración final

**Files:** ninguno nuevo.

- [ ] **Step 14.1: Verificar que el proceso arranca sin crash inmediato**

```bash
WHATSAPP_MEDIA_DIR=C:/whatsapp-media WHATSAPP_DB_PATH=./data bun run src/index.ts &
sleep 3
kill %1 2>/dev/null || true
```

Resultado esperado: proceso arranca sin mensaje de error fatal en stderr.

- [ ] **Step 14.2: Crear carpeta de media**

```bash
mkdir -p "C:/whatsapp-media"
```

- [ ] **Step 14.3: Configurar Claude Desktop**

Abrir `%APPDATA%\Claude\claude_desktop_config.json` y agregar:

```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "bun",
      "args": ["run", "C:\\dev\\mcp\\whatsapp-mcp\\src\\index.ts"],
      "env": {
        "WHATSAPP_MEDIA_DIR": "C:\\whatsapp-media",
        "WHATSAPP_DB_PATH": "C:\\dev\\mcp\\whatsapp-mcp\\data"
      }
    }
  }
}
```

- [ ] **Step 14.4: Reiniciar Claude Desktop y verificar que aparecen los 8 tools**

Los tools `get_connection_status`, `connect_qr`, `connect_phone`, `list_chats`, `get_messages`, `get_message`, `search_messages`, `download_attachment` deben aparecer disponibles.

- [ ] **Step 14.5: Autenticarse vía QR**

Pedirle a Claude: *"Usá el tool connect_qr para conectar WhatsApp"*
1. Claude llama `connect_qr` → aparece QR ASCII en la conversación
2. Abrir WhatsApp en el teléfono → Dispositivos vinculados → Vincular dispositivo → escanear QR
3. Llamar `get_connection_status` → debe devolver `{"status":"connected"}`
4. Llamar `list_chats` → debe devolver JSON con los chats sincronizados

- [ ] **Step 14.6: Commit final**

```bash
git add .
git commit -m "chore: finalize whatsapp-mcp TypeScript/Bun implementation"
```
