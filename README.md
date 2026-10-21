# WhatsApp MCP

Servidor MCP (Model Context Protocol) de solo lectura que conecta con WhatsApp Web a través de Baileys. Expone herramientas para leer mensajes, listar chats y descargar adjuntos. Ejecutable en Bun.

## Requisitos

- Bun 1.x

## Instalación

```bash
bun install
```

## Configuración

El servidor requiere variables de entorno para funcionar. Cópialas en un archivo `.env` en la raíz del proyecto:

| Variable | Requerida | Valor por defecto | Descripción |
|----------|-----------|-------------------|-------------|
| `WHATSAPP_MEDIA_DIR` | Sí | — | Ruta local donde se descargan los adjuntos. Ej: `C:/whatsapp-media` |
| `WHATSAPP_DB_PATH` | No | `./data` | Ruta de la base de datos SQLite (con sesiones y mensajes) |
| `LOG_LEVEL` | No | `info` | Nivel de logging: `debug`, `info`, `warn`, `error` |

**Ejemplo `.env`:**
```
WHATSAPP_MEDIA_DIR=C:/whatsapp-media
WHATSAPP_DB_PATH=./data
LOG_LEVEL=info
```

## Uso / Ejecución

El servidor actúa como un servidor MCP por stdio, pensado para integración con Claude Desktop.

```bash
# Opción 1: Script start
bun start

# Opción 2: Ejecutar directamente
bun run src/index.ts
```

## Configuración en Claude Desktop

Agregá la siguiente sección a tu archivo `claude_desktop_config.json` (usualmente en `~/.claude/claude_desktop_config.json`):

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

**Nota:** Reemplazá las rutas según tus directorios locales. En Windows usá doble backslash (`\\`).

## Tools disponibles

El servidor expone 8 herramientas de solo lectura:

| Tool | Descripción |
|------|-------------|
| `get_connection_status` | Devuelve el estado actual de la conexión con WhatsApp |
| `connect_qr` | Inicia conexión con WhatsApp vía QR. Devuelve el QR en ASCII para escanearlo con el teléfono. |
| `connect_phone` | Inicia conexión vía número de teléfono. Devuelve código de 8 caracteres para ingresar en la app. |
| `list_chats` | Lista todos los chats de WhatsApp (individuales y grupos) ordenados por actividad reciente |
| `get_messages` | Devuelve mensajes de un chat con paginación y filtro por fecha |
| `get_message` | Obtiene un mensaje específico por ID |
| `search_messages` | Busca mensajes por texto en todos los chats o en uno específico |
| `download_attachment` | Descarga el adjunto de un mensaje a la carpeta local y devuelve la ruta en disco |

## Autenticación

### Primera conexión

El servidor requiere vincular un dispositivo WhatsApp antes de poder leer mensajes.

**Opción 1: Via QR (recomendado)**
- Llamá a `connect_qr`. El servidor genera un código QR en ASCII.
- Abrí WhatsApp en tu teléfono > Dispositivos vinculados > Vincular un dispositivo.
- Escaneá el QR que aparece en la consola.

**Opción 2: Via teléfono**
- Llamá a `connect_phone` con tu número internacional (sin +, ej: `5491112345678`).
- El servidor devuelve un código de 8 caracteres.
- Abrí WhatsApp > Dispositivos vinculados > Vincular con número de teléfono e ingresá el código.

### Persistencia de sesión

Una vez vinculado, el servidor guarda la sesión en `<WHATSAPP_DB_PATH>/auth/`. En reconexiones posteriores, se autentica automáticamente sin necesidad de escanear el QR nuevamente.

## Tests

```bash
bun test
```

## Nota

Este servidor es **solo lectura**. No envía mensajes ni realiza acciones de escritura en WhatsApp. Está diseñado para integración con Claude como herramienta de lectura e investigación de conversaciones.
