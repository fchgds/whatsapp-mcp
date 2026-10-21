# WhatsApp MCP — Especificación de diseño
**Fecha:** 2026-06-19

## Resumen

Servidor MCP escrito en Go puro que conecta a WhatsApp Web mediante el protocolo multi-device (whatsmeow) y expone herramientas de solo lectura a un asistente de IA (Claude u otro). Permite leer mensajes de todos los chats y descargar adjuntos (fotos, videos, audios, documentos) a una carpeta local configurable.

---

## Arquitectura general

Un único binario Go con tres responsabilidades internas:

```
┌─────────────────────────────────────────────────────┐
│                  whatsapp-mcp (Go)                  │
│                                                     │
│  ┌─────────────┐    ┌──────────────┐    ┌────────┐  │
│  │  WhatsApp   │    │   Message    │    │  MCP   │  │
│  │  Client     │───▶│   Store      │◀───│ Server │  │
│  │ (whatsmeow) │    │  (SQLite)    │    │(mcp-go)│  │
│  └─────────────┘    └──────────────┘    └────────┘  │
│         │                                    │      │
│         ▼                                    ▼      │
│   WhatsApp Web              AI Assistant (Claude)   │
│   (protocolo WA)            vía stdio/MCP           │
└─────────────────────────────────────────────────────┘
```

1. **WhatsApp Client** — `whatsmeow` gestiona la conexión al protocolo multi-device, autenticación (QR o número de teléfono), recepción de mensajes en tiempo real y persistencia de sesión.
2. **Message Store** — SQLite para sesión (`session.db` gestionada por whatsmeow) y para mensajes/adjuntos (`messages.db` gestionada por nosotros).
3. **MCP Server** — `mcp-go` comunica el servidor con el asistente de IA vía `stdio`.

---

## Tools MCP expuestas

| Tool | Descripción |
|------|-------------|
| `get_connection_status` | Estado actual: desconectado / esperando QR / esperando código de teléfono / conectado |
| `connect_qr` | Inicia conexión por QR; devuelve el QR como texto ASCII |
| `connect_phone` | Inicia conexión por número de teléfono; devuelve el código de vinculación de 8 dígitos |
| `list_chats` | Lista todos los chats (individuales y grupos) con nombre, JID y timestamp del último mensaje |
| `get_messages` | Mensajes de un chat dado; parámetros: `chat_jid`, `limit`, `offset`, `from_timestamp`, `to_timestamp` |
| `get_message` | Mensaje específico por ID con contenido completo y metadatos |
| `search_messages` | Búsqueda de texto en todos los chats o en uno específico |
| `download_attachment` | Descarga un adjunto a `media_dir` y devuelve la ruta en disco |

**Payload de mensaje devuelto:**
```json
{
  "id": "ABC123",
  "chat_jid": "1234567890@s.whatsapp.net",
  "sender_jid": "1234567890@s.whatsapp.net",
  "timestamp": "2026-06-19T10:00:00Z",
  "type": "image",
  "body": "Mira esto",
  "has_attachment": true,
  "attachment": {
    "mime_type": "image/jpeg",
    "file_size": 204800,
    "is_downloaded": false,
    "file_path": ""
  }
}
```

---

## Almacenamiento

### session.db
Gestionada directamente por whatsmeow. Contiene credenciales de sesión, claves de cifrado y estado del dispositivo.

### messages.db
Gestionada por el proyecto.

```sql
CREATE TABLE chats (
    jid                    TEXT PRIMARY KEY,
    name                   TEXT,
    last_message_timestamp INTEGER,
    unread_count           INTEGER DEFAULT 0
);

CREATE TABLE messages (
    id              TEXT PRIMARY KEY,
    chat_jid        TEXT NOT NULL,
    sender_jid      TEXT NOT NULL,
    timestamp       INTEGER NOT NULL,
    type            TEXT NOT NULL,  -- text/image/video/audio/document/sticker
    body            TEXT,
    has_attachment  INTEGER DEFAULT 0,
    is_downloaded   INTEGER DEFAULT 0,
    FOREIGN KEY (chat_jid) REFERENCES chats(jid)
);

CREATE TABLE attachments (
    message_id TEXT PRIMARY KEY,
    file_path  TEXT,
    mime_type  TEXT,
    file_size  INTEGER,
    sha256     TEXT,
    FOREIGN KEY (message_id) REFERENCES messages(id)
);
```

### Descarga de adjuntos

Los adjuntos **no se descargan automáticamente**. Solo se descargan al llamar `download_attachment`. Ruta de guardado: `<media_dir>/<chat_jid>/<message_id>.<ext>`. Los links de media de WhatsApp tienen TTL; si expiraron se devuelve error descriptivo.

---

## Configuración

Archivo `config.yaml` en el directorio del binario:

```yaml
media_dir: "C:/whatsapp-media"   # carpeta destino para adjuntos
db_path:   "./data"              # directorio para session.db y messages.db
log_level: "info"                # debug | info | warn | error
```

---

## Estructura del proyecto

```
whatsapp-mcp/
├── cmd/
│   └── main.go                  # punto de entrada, carga config, inicializa todo
├── internal/
│   ├── whatsapp/
│   │   ├── client.go            # conexión whatsmeow, QR, número de teléfono
│   │   └── handlers.go          # recepción de mensajes en tiempo real → store
│   ├── store/
│   │   ├── db.go                # inicialización SQLite, migraciones
│   │   ├── chats.go             # queries de chats
│   │   ├── messages.go          # queries de mensajes
│   │   └── attachments.go       # registro de adjuntos descargados
│   └── mcp/
│       ├── server.go            # inicialización mcp-go, registro de tools
│       └── tools.go             # implementación de cada tool MCP
├── config.yaml
├── go.mod
└── go.sum
```

---

## Dependencias principales

| Módulo | Propósito |
|--------|-----------|
| `go.mau.fi/whatsmeow` | Cliente WhatsApp Web (protocolo multi-device) |
| `github.com/mark3labs/mcp-go` | Servidor MCP vía stdio |
| `modernc.org/sqlite` | SQLite puro Go (sin CGO — compila en Windows sin toolchain adicional) |
| `gopkg.in/yaml.v3` | Parseo de config.yaml |

---

## Manejo de errores

- **Desconexión de WhatsApp:** whatsmeow reintenta automáticamente con backoff exponencial. Durante la desconexión, los tools de lectura devuelven los datos cacheados en SQLite.
- **Tool llamado sin conexión:** devuelve error MCP descriptivo con el estado actual.
- **Link de media expirado:** `download_attachment` devuelve error con mensaje claro.
- **Errores de SQLite:** se loggean y se propagan como errores MCP.

---

## Restricciones

- **Solo lectura** — no se envían mensajes ni se modifica nada en WhatsApp.
- **Sesión persistida** — al reiniciar el binario, whatsmeow restaura la sesión desde `session.db` sin requerir nuevo QR.
- **Historial sincronizado** — al conectar por primera vez, WhatsApp sincroniza automáticamente los mensajes recientes (típicamente ~90 días según la configuración del teléfono). Todo lo recibido desde ese momento en adelante queda persistido en SQLite.
- **Sin descarga automática** — los adjuntos solo se descargan bajo demanda vía `download_attachment`.
