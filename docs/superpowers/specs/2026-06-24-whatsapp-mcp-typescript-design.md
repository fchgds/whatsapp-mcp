# WhatsApp MCP — Especificación TypeScript
**Fecha:** 2026-06-24

## Resumen

Servidor MCP escrito en TypeScript (Bun) que conecta a WhatsApp Web mediante el protocolo multi-device (Baileys) y expone herramientas de solo lectura a un asistente de IA (Claude u otro). Permite leer mensajes de todos los chats y descargar adjuntos a una carpeta local configurable. Migración del diseño Go original manteniendo las mismas tools MCP y el mismo schema de base de datos.

---

## Stack

| Elemento | Elección |
|----------|----------|
| Runtime | Bun |
| Cliente WhatsApp | `@whiskeysockets/baileys` |
| Servidor MCP | `@modelcontextprotocol/sdk` |
| SQLite | `bun:sqlite` (built-in, sin bindings nativos) |
| Auth state | `useMultiFileAuthState("data/auth")` |
| Configuración | Variables de entorno (`.env` / `env` en claude_desktop_config.json) |
| Transport MCP | stdio |

---

## Arquitectura general

```
src/
├── index.ts                  ← entry point: carga config, inicializa capas, arranca MCP
├── config.ts                 ← lee y valida variables de entorno
├── store/
│   ├── db.ts                 ← abre bun:sqlite, aplica schema, exporta DB singleton
│   ├── chats.ts              ← upsertChat, listChats
│   ├── messages.ts           ← insertMessage, getMessages, getMessage, searchMessages
│   └── attachments.ts        ← upsertAttachment, getAttachment, markDownloaded
├── whatsapp/
│   ├── client.ts             ← init Baileys, QR, phone link, status, reconexión
│   ├── handlers.ts           ← event handlers: mensajes + HistorySync → store
│   └── media.ts              ← descarga de adjuntos por tipo de mensaje
└── mcp/
    ├── server.ts             ← McpServer, StdioServerTransport, registro de tools
    ├── tools-connection.ts   ← get_connection_status, connect_qr, connect_phone
    ├── tools-messages.ts     ← list_chats, get_messages, get_message, search_messages
    └── tools-attachments.ts  ← download_attachment
```

Un único proceso Bun. Las capas se comunican por inyección directa de dependencias (funciones reciben `db` y `waState` como parámetros), sin estado global excepto el socket de Baileys.

---

## Tools MCP expuestas

Idénticas al diseño Go:

| Tool | Descripción |
|------|-------------|
| `get_connection_status` | Estado actual: disconnected / connecting / waiting_qr / waiting_phone / connected |
| `connect_qr` | Inicia conexión por QR; devuelve el QR como texto ASCII |
| `connect_phone` | Inicia conexión por número de teléfono; devuelve el código de vinculación de 8 caracteres |
| `list_chats` | Lista todos los chats (individuales y grupos) ordenados por actividad reciente |
| `get_messages` | Mensajes de un chat dado; parámetros: `chat_jid`, `limit`, `offset`, `from_timestamp`, `to_timestamp` |
| `get_message` | Mensaje específico por ID |
| `search_messages` | Búsqueda de texto en todos los chats o en uno específico |
| `download_attachment` | Descarga un adjunto a `WHATSAPP_MEDIA_DIR` y devuelve la ruta en disco |

---

## Capa WhatsApp (client.ts)

```typescript
type Status = "disconnected" | "connecting" | "waiting_qr" | "waiting_phone" | "connected"

export const waState: { status: Status; socket: WASocket | null }

export async function initClient(dbPath: string): Promise<void>
export async function connectQR(): Promise<string>          // devuelve QR ASCII
export async function connectPhone(phone: string): Promise<string>  // devuelve código 8 chars
export async function connectExisting(): Promise<void>
export function getStatus(): Status
export async function downloadMedia(rawProto: Uint8Array, type: string): Promise<Buffer>
```

**Auth state:** `useMultiFileAuthState("data/auth")` — almacena credenciales en `data/auth/*.json`. Al arrancar, si existe `data/auth/creds.json`, se reconecta automáticamente sin pedir QR.

**handlers.ts** registra listeners en el socket de Baileys:
- `connection.update` → actualiza `waState.status`
- `messages.upsert` → persiste mensajes nuevos en store
- `messaging-history.set` → persiste historial al conectar por primera vez

**media.ts** usa `downloadMediaMessage()` de Baileys para descargar bytes. Los mensajes con adjunto almacenan el sub-mensaje proto serializado con `proto.encode()` en la columna `raw_media_proto BLOB` para permitir re-descarga posterior.

---

## Capa Store (SQLite)

**Schema** — idéntico al diseño Go:

```sql
CREATE TABLE IF NOT EXISTS chats (
    jid                    TEXT PRIMARY KEY,
    name                   TEXT NOT NULL DEFAULT '',
    last_message_timestamp INTEGER NOT NULL DEFAULT 0,
    unread_count           INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS messages (
    id              TEXT PRIMARY KEY,
    chat_jid        TEXT NOT NULL,
    sender_jid      TEXT NOT NULL,
    timestamp       INTEGER NOT NULL,
    type            TEXT NOT NULL,
    body            TEXT NOT NULL DEFAULT '',
    has_attachment  INTEGER NOT NULL DEFAULT 0,
    is_downloaded   INTEGER NOT NULL DEFAULT 0,
    raw_media_proto BLOB,
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
```

**Interfaz funcional** — cada archivo recibe `db: Database` como parámetro:

```typescript
// chats.ts
export function upsertChat(db: Database, chat: Chat): void
export function listChats(db: Database): Chat[]

// messages.ts
export function insertMessage(db: Database, msg: Message): void
export function getMessages(db: Database, opts: GetMessagesOpts): Message[]
export function getMessage(db: Database, id: string): Message | null
export function searchMessages(db: Database, query: string, chatJID?: string): Message[]

// attachments.ts
export function upsertAttachment(db: Database, att: Attachment): void
export function getAttachment(db: Database, messageID: string): Attachment | null
export function markDownloaded(db: Database, messageID: string, filePath: string): void
```

`bun:sqlite` opera en modo sincrónico — sin callbacks ni Promises en el store, que simplifica el código de toda la capa.

---

## Capa MCP (server.ts)

```typescript
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js"
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js"

export async function startMCP(db: Database, wa: typeof waState): Promise<void> {
  const server = new McpServer({ name: "whatsapp-mcp", version: "1.0.0" })
  registerConnectionTools(server, wa)
  registerMessageTools(server, db)
  registerAttachmentTools(server, db, wa)
  await server.connect(new StdioServerTransport())
}
```

Los tools usan `server.tool(name, schema, handler)` de `@modelcontextprotocol/sdk`. Los schemas de parámetros se definen con Zod (incluido en el SDK).

---

## Punto de entrada (index.ts)

Secuencia de arranque:

1. Cargar y validar variables de entorno (`config.ts`)
2. `openDB(config.dbPath)` → inicializa SQLite con schema
3. `initClient(config.dbPath)` → carga auth state de Baileys
4. `registerHandlers(db)` → conecta eventos Baileys → store
5. Si existe sesión guardada → `connectExisting()` automáticamente
6. `startMCP(db, waState)` → bloquea en stdio

---

## Configuración

**Variables de entorno:**

| Variable | Descripción | Default |
|----------|-------------|---------|
| `WHATSAPP_MEDIA_DIR` | Carpeta para adjuntos descargados | **requerida** |
| `WHATSAPP_DB_PATH` | Directorio para SQLite y auth state | `./data` |
| `LOG_LEVEL` | Nivel de log: debug / info / warn / error | `info` |

**Claude Desktop** (`%APPDATA%\Claude\claude_desktop_config.json`):

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

---

## Dependencias

```json
{
  "dependencies": {
    "@whiskeysockets/baileys": "latest",
    "@modelcontextprotocol/sdk": "latest",
    "qrcode-terminal": "^0.12.0",
    "pino": "^9.0.0"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "@types/node": "^20.0.0",
    "@types/qrcode-terminal": "^0.12.2"
  }
}
```

---

## Manejo de errores

- **Desconexión de WhatsApp:** Baileys reintenta con backoff exponencial. Durante la desconexión, los tools de lectura devuelven datos cacheados desde SQLite.
- **Tool llamado sin conexión:** devuelve error MCP con el status actual en el mensaje.
- **Link de media expirado:** `download_attachment` captura el error de Baileys y devuelve mensaje descriptivo.
- **Auth state inválido:** si las credenciales están corruptas, se eliminan y se solicita nuevo QR.

---

## Restricciones

- **Solo lectura** — no se envían mensajes ni se modifica nada en WhatsApp.
- **Sesión persistida** — `data/auth/creds.json` permite reconectar sin nuevo QR.
- **Historial sincronizado** — al conectar por primera vez, Baileys recibe el historial vía `messaging-history.set` (típicamente ~90 días).
- **Sin descarga automática** — los adjuntos solo se descargan bajo demanda vía `download_attachment`.
