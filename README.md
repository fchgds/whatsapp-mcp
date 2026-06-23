# whatsapp-mcp

MCP en Go para leer WhatsApp (como WhatsApp Web, vía QR) y descargar adjuntos. Solo lectura.

## Requisitos
- Go 1.26+ (`winget install GoLang.Go`)
- Windows (probado), sin compilador C (SQLite es puro Go)

## Build
```powershell
go build -o whatsapp-daemon.exe ./cmd/daemon
go build -o whatsapp-mcp.exe ./cmd/mcp
```

## Configuración
Copiá `config.example.json` a `config.json` y poné un token aleatorio en `ipc_token`.

## Linkeo (una sola vez)
```powershell
.\whatsapp-daemon.exe
```
Escaneá el QR desde WhatsApp → *Dispositivos vinculados*. Dejá el daemon corriendo
en segundo plano (o configuralo en el Programador de tareas para que arranque con Windows):
es quien mantiene la sesión y captura los mensajes.

## Registrar el MCP en Claude
En la config de MCP del cliente (ej. Claude Desktop / Claude Code), agregá:
```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "C:\\dev\\mcp\\whatsapp-mcp\\whatsapp-mcp.exe",
      "cwd": "C:\\dev\\mcp\\whatsapp-mcp"
    }
  }
}
```
> `cwd` debe ser la carpeta del proyecto para que encuentre `config.json` y `data/`.

## Herramientas
- `get_connection_status` · `search_contacts` · `list_chats` · `get_messages` · `list_media` · `download_media`

## Privacidad
`data/messages.db` guarda el texto de tus mensajes en claro, sólo en tu máquina.
Uso no oficial del protocolo de WhatsApp; usalo con moderación.
