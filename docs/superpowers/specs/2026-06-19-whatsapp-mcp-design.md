# Diseño: `whatsapp-mcp` — MCP de WhatsApp (solo lectura + descargas)

- **Fecha:** 2026-06-19
- **Estado:** Aprobado (pendiente de plan de implementación)
- **Plataforma objetivo:** Windows 11 (uso personal, un solo usuario)

## 1. Objetivo

Exponer un servidor **MCP** que permita a Claude operar sobre la cuenta de WhatsApp
del usuario "como WhatsApp Web", para casos de uso tipo:

- *"Resúmeme lo que hablé con Fulano."*
- *"Descargame los archivos de ese chat a `C:\...\carpeta`."*

Alcance: **solo lectura + descargas**. No envía mensajes, no marca como leído, no reacciona.

## 2. Decisiones tomadas (brainstorming)

| Decisión | Elección | Motivo |
|---|---|---|
| Alcance | Solo lectura + descargas | Más simple, menor superficie/riesgo para la cuenta |
| Stack | **Todo-Go**: whatsmeow + Go MCP SDK | whatsmeow es la librería más mantenida (commits esta semana, grado producción); un solo lenguaje; sin navegador; liviano |
| Historial | **Captura persistente** (daemon + SQLite) | Permite resumir conversaciones aunque Claude no esté abierto en ese momento; consultas rápidas |

### Stacks descartados
- **Baileys (TS):** la 7.0 está en RC; preferencia del usuario por algo "más vivo".
- **whatsapp-web.js (TS):** mantenido, pero pesado (Chromium headless) y más frágil ante cambios de la web de WhatsApp.
- **whatsmeow (Go) + MCP en Python:** dos lenguajes/procesos; el repo de referencia (`lharries/whatsapp-mcp`) está abandonado (último commit jul-2025).

## 3. Aviso importante (no oficial)

WhatsApp **no tiene API oficial** para cuentas personales. Esto reimplementa el protocolo
multi-dispositivo (vía QR). Para uso personal y moderado es estable y habitual, pero el
riesgo de baneo de la cuenta **no es exactamente cero**.

La base de datos local guarda el **contenido de los mensajes en texto plano**, únicamente
en la máquina del usuario.

## 4. Arquitectura

Un solo proyecto Go con **dos procesos** que comparten almacenamiento SQLite local:

```
┌─────────────────┐         ┌──────────────────────────┐
│  Claude (cliente│  stdio  │  MCP server (whatsapp-mcp)│
│   MCP)          │ ──────► │  - lee SQLite (consultas) │
└─────────────────┘         │  - llama al daemon (live) │
                            └─────────┬────────────────┘
                              lee DB  │  HTTP 127.0.0.1 (token)
                            ┌─────────▼────────────────┐
                            │  Daemon (whatsapp-daemon) │
                            │  - cliente whatsmeow vivo │
                            │  - auth + reconexión      │
                            │  - ingesta → SQLite       │
                            │  - descarga media (live)  │
                            └─────────┬────────────────┘
                              WhatsApp │ multi-device (QR)
                            ┌─────────▼─────┐   ┌──────────────┐
                            │ messages.db   │   │ session.db    │
                            │ (nuestro)     │   │ (whatsmeow)   │
                            └───────────────┘   └──────────────┘
```

### Daemon (`whatsapp-daemon.exe`)
Proceso siempre-activo, **único** que habla con WhatsApp.
- Mantiene el cliente whatsmeow y el socket vivo; gestiona reconexión con backoff.
- Ingiere mensajes/contactos/chats a `messages.db` a medida que llegan.
- Expone una mini-API HTTP **solo en `127.0.0.1`**, protegida con token, para acciones "vivas":
  estado de conexión, QR actual, y descarga de media (que requiere el cliente vivo).

### MCP server (`whatsapp-mcp.exe`)
Proceso liviano que Claude arranca por **stdio**.
- Consultas (buscar contacto, listar chats, leer mensajes): **leen SQLite directo** (rápido,
  no dependen de que el daemon responda).
- Acciones vivas (descargar media, estado/QR): las **delega al daemon** por HTTP local.

**Por qué dos procesos:** que la captura siga ocurriendo aunque Claude no esté abierto, y que
cada pieza se entienda y testee de forma aislada. Una sola sesión de whatsmeow puede estar
activa por dispositivo, así que el daemon es el dueño único de la conexión.

## 5. Conexión y QR (linking)

- La **sesión** la persiste el `sqlstore` de whatsmeow en `data/session.db`. Se linkea **una sola vez**.
- **Primer linkeo:** el daemon, al arrancar sin sesión válida, abre `client.GetQRChannel(ctx)`
  antes de `client.Connect()` y **pinta el QR en su consola** (`qrterminal`). El usuario escanea
  desde WhatsApp → *Dispositivos vinculados*. Al vincularse, la sesión queda guardada.
- **Uso normal:** el daemon corre en segundo plano y **reconecta solo** con la sesión guardada.
- **Logout / sesión inválida:** marca estado `needs_qr`; hay que volver a linkear.

## 6. Almacenamiento (SQLite vía `modernc.org/sqlite`, puro Go, sin CGO)

Dos archivos SQLite (driver pure-Go → compila en Windows sin gcc):

- `data/session.db` — sesión de whatsmeow (esquema gestionado por su `sqlstore`).
- `data/messages.db` — nuestras tablas:
  - `chats` — `jid`, `name`, `type` (individual/group), `last_message_text`, `last_message_ts`.
  - `contacts` — `jid`, `name`/`push_name`, `phone`.
  - `messages` — `id`, `chat_jid`, `sender_jid`, `ts`, `type` (text/image/audio/video/document/sticker),
    `text`, y metadatos de media necesarios para descargar (`mimetype`, `filename`, `size`,
    y las claves/path que whatsmeow necesita: media key, direct path, file enc/sha, etc.).

La media **no** se guarda al ingerir; se descarga **bajo demanda** a la carpeta que se indique.

### Ingesta
`client.AddEventHandler` filtrando:
- `*events.Message` → mensajes en vivo.
- `*events.HistorySync` → historial que WhatsApp envía al linkear.
- `*events.Contact` / push names → poblar/actualizar `contacts`.

## 7. Herramientas MCP

| Herramienta | Para qué |
|---|---|
| `get_connection_status` | ¿Conectado? ¿`needs_qr`? número vinculado |
| `search_contacts(query)` | Buscar contacto/chat por nombre o número → candidatos con su JID |
| `list_chats(limit?, query?)` | Chats recientes con vista previa del último mensaje |
| `get_messages(chat, limit?, before?, after?)` | Mensajes de un chat (por nombre o JID). Base de "resumir lo que hablé con X" |
| `list_media(chat, types?, limit?)` | Listar adjuntos disponibles en un chat antes de bajarlos |
| `download_media(chat, dest_folder, types?, date_range?, limit?)` | Descargar adjuntos a la carpeta indicada → devuelve rutas guardadas |

- `chat` acepta **nombre o JID**. Si el nombre es ambiguo, `search_contacts`/`get_messages`
  devuelven **candidatos** y **no adivinan**.
- `download_media` se ejecuta vía el daemon (necesita el cliente vivo); escribe los archivos con
  extensión derivada del mimetype y devuelve la lista de rutas guardadas.

## 8. Flujos de uso

**"Resúmeme lo que hablé con Fulano"**
1. `search_contacts("Fulano")` → JID
2. `get_messages(jid, limit=N)` → historial de texto (de SQLite)
3. Claude resume

**"Descargame los archivos de ese chat a `C:\...\carpeta`"**
1. (ya tiene el JID) `list_media(jid)` → ve qué hay
2. `download_media(jid, dest_folder="C:\...\carpeta", types=[...])` → MCP → daemon → descarga y
   escribe archivos → devuelve rutas

## 9. Manejo de errores

- **No linkeado** → tools devuelven error claro: "ejecutá el daemon y escaneá el QR".
- **Daemon caído** → consultas de lectura siguen funcionando desde SQLite; descarga/estado avisan
  "daemon no disponible".
- **Logout (sesión inválida)** → estado `needs_qr`; requiere re-linkear.
- **Media expirada en CDN** → el daemon intenta re-subida vía cliente; si falla, error claro.
- **Contacto ambiguo** → devuelve candidatos, no adivina.

## 10. Comunicación daemon ↔ MCP (IPC)

- HTTP en `127.0.0.1` con puerto y **token** en config (`config.json` o `.env`).
- Endpoints mínimos: `/status`, `/qr`, `/download`.
- El token evita que otro proceso local invoque acciones vivas.

## 11. Testing

- **Unit:** capa de store (SQLite en archivo temporal), normalización de mensajes, handlers de
  tools con store sembrado y cliente-whatsmeow mockeado **detrás de una interfaz**.
- **Integración:** alimentar eventos de whatsmeow grabados → ingesta → verificar filas en DB;
  round-trip de tools sobre una DB sembrada.
- **Manual (documentado):** flujo de linkeo (QR) y una descarga real. No se puede e2e contra
  WhatsApp real en CI.

## 12. Dependencias (versiones verificadas 2026-06-19)

| Componente | Módulo | Versión |
|---|---|---|
| Lenguaje | Go | `1.26.4` *(a instalar)* |
| WhatsApp | `go.mau.fi/whatsmeow` | latest (`v0.0.0-20260616...`) |
| MCP | `github.com/modelcontextprotocol/go-sdk` | `v1.6.1` |
| SQLite | `modernc.org/sqlite` | `v1.52.0` (puro Go, sin CGO) |
| QR terminal | `github.com/mdp/qrterminal/v3` | `v3.2.1` |

## 13. Prerequisitos

- Instalar **Go 1.26.4** en Windows: `winget install GoLang.Go` o el MSI de https://go.dev/dl/.
- Un cliente MCP (Claude Desktop / Claude Code) configurado para arrancar `whatsapp-mcp.exe` por stdio.

## 14. Estructura de proyecto propuesta

```
whatsapp-mcp/
  cmd/
    daemon/main.go     # whatsapp-daemon.exe
    mcp/main.go        # whatsapp-mcp.exe
  internal/
    wa/                # wrapper whatsmeow (interfaz + impl)
    store/             # SQLite (modernc): escritura (daemon) y lectura (mcp)
    ingest/            # event handlers → store
    ipc/               # cliente/servidor HTTP localhost (token)
    tools/             # handlers de las 6 herramientas MCP
    config/            # carga de config (puerto, token, rutas)
  data/                # session.db, messages.db, (gitignored)
  docs/superpowers/specs/
  go.mod
```
