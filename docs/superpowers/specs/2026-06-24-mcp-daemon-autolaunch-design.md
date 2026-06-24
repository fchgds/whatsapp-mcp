# Diseño: auto-arranque del daemon desde el MCP + aviso de vinculación

Fecha: 2026-06-24
Branch: feature/whatsapp-mcp-impl

## Problema

Hoy el proyecto son dos binarios separados:

- `whatsapp-daemon.exe`: whatsmeow + SQLite + servidor IPC HTTP en `127.0.0.1:8377`. Imprime el QR de vinculación en *su* terminal.
- `whatsapp-mcp.exe`: servidor MCP por stdio que Claude Code lanza. Lee la misma SQLite y delega en el daemon vía IPC para `status` y `download`.

El daemon hay que arrancarlo a mano. Si no está corriendo, las tools que lo necesitan fallan. Y si el daemon necesita vincularse (QR), el usuario del MCP no se entera porque el QR vive en una terminal que puede no estar visible.

Objetivo: que el MCP **levante el daemon si no responde** y que, cuando haga falta **vincularse**, el usuario del MCP se entere por el chat y pueda escanear el QR.

## Decisiones tomadas

1. **Surfacing del QR**: ventana de terminal visible. El MCP lanza el daemon con su propia consola, donde aparece el QR ASCII que el daemon ya renderiza. El asistente avisa en el chat que mire esa ventana.
2. **Ciclo de vida**: el daemon muere con el MCP.
3. **Disparo**: lazy — el daemon se lanza recién cuando se llama a una tool (no al iniciar el MCP), así no abre ventana en sesiones donde no se usa WhatsApp.
4. **Robustez del kill en Windows**: Job Object con `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, para que el SO mate al daemon aunque el MCP sea forzado a cerrar.

## Arquitectura

```
Claude Code → whatsapp-mcp.exe (stdio)
                  │
                  ├─ Launcher.EnsureRunning()  ← al inicio de cada tool
                  │     1. ¿/status responde? → sí: usar
                  │     2. no: spawn whatsapp-daemon.exe (consola nueva visible + Job Object)
                  │     3. poll /status hasta Connected | NeedsQR | timeout
                  │
                  └─ tools → IPC → daemon (whatsmeow + SQLite)
```

El daemon **no cambia** su lógica de ingesta ni su render de QR. Solo recibe, desde el MCP, una consola propia y un Job Object que controla su ciclo de vida.

## Componentes nuevos

### `internal/launcher` (lado MCP)

```go
type Launcher struct {
    DaemonPath string        // ruta al whatsapp-daemon.exe
    WorkDir    string        // carpeta donde vive config.json (Dir del proceso)
    Client     *ipc.Client   // para chequear /status
    mu         sync.Mutex    // single-flight: dos tools en paralelo no duplican
    job        windows.Handle // Job Object
    cmd        *exec.Cmd
    started    bool
}
```

- `EnsureRunning(ctx) (ipc.Status, error)`:
  1. Intenta `Client.Status(ctx)` con timeout corto. Si responde, devuelve el estado (que puede seguir siendo `NeedsQR`).
  2. Toma el mutex y **re-chequea** status (otro llamado pudo haberlo arrancado).
  3. Spawnea el daemon (ver flujo Windows abajo).
  4. Poolea `/status` hasta `Connected` o `NeedsQR` o timeout (~20s).
  5. Devuelve el estado final.
- `Close()`: cierra el handle del Job Object → el SO mata al daemon. Se llama con `defer` tras `server.Run`.

Código Windows-específico (Job Object + `CREATE_NEW_CONSOLE`) va en `launcher_windows.go` con build tag `//go:build windows`. Si se necesita compilar en otra plataforma, un stub en `launcher_other.go` devuelve "no soportado".

### `config`

- Campo opcional `daemon_path` en `config.json`.
- Default: `whatsapp-daemon.exe` en la misma carpeta que el ejecutable del MCP (`os.Executable()`).
- El daemon se lanza con `cmd.Dir =` carpeta del `config.json`, para que encuentre `config.json` y `data/`.

## Flujo de arranque y vinculación

1. Primera tool de la sesión → `EnsureRunning`.
2. `/status` no responde → se crea el Job Object con `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, se hace `CreateProcess` con `CREATE_NEW_CONSOLE` (ventana visible) y `cmd.Dir = WorkDir`, y se asigna el proceso al job con `AssignProcessToJobObject`.
3. El MCP poolea `/status` (timeout ~20s):
   - `Connected=true` → la tool sigue normalmente.
   - `NeedsQR=true` → la tool corta y devuelve un mensaje claro: *"Se abrió una ventana de WhatsApp; escaneá el QR desde WhatsApp → Dispositivos vinculados y reintentá."*
   - timeout → error `"no pude arrancar el daemon"`.
4. Al cerrar el MCP (fin de `server.Run`, vía `defer Close()`), se cierra el job → el daemon muere. Si el MCP es forzado (kill, crash), el Job Object igual lo mata.

### Nota sobre Job Object + CREATE_NEW_CONSOLE

`CREATE_NEW_CONSOLE` y la asignación al Job Object conviven (flags de `SysProcAttr.CreationFlags` + `AssignProcessToJobObject` explícito). No se debe usar `CREATE_BREAKAWAY_FROM_JOB`. En Windows moderno los jobs anidados están permitidos, así que asignar el daemon al job del MCP no debería fallar aunque el MCP ya esté dentro de un job (p. ej. lanzado por Claude Code).

## Cambios en tools y status

- Helper `ensureDaemon(ctx)` al inicio de cada handler de tool. Si el estado no es `Connected`, la tool devuelve el mensaje de vinculación en vez de datos vacíos o parciales.
- Aplica a las **6 tools** (todas son de WhatsApp), coherente con "lazy".
- `get_connection_status` (`StatusOut`) gana un campo `message` legible:
  - `"vinculado como +54…"` cuando `Connected`.
  - `"necesita QR: mirá la ventana que se abrió y escaneá desde WhatsApp → Dispositivos vinculados"` cuando `NeedsQR`.
  - `"arrancando…"` durante el poll.

## Alcance / qué NO se toca

- El daemon sigue igual: misma ingesta, mismo render de QR ASCII. Solo recibe consola propia y Job Object desde el MCP.
- No hay reconexión automática ni reintentos sofisticados más allá del poll inicial.
- No se cachean datos para responder tools de solo-lectura cuando no está vinculado: si no está `Connected`, se devuelve el mensaje de vinculación.

## Criterios de éxito

- Con el daemon apagado, llamar a cualquier tool del MCP arranca `whatsapp-daemon.exe` en una ventana visible.
- Si la sesión no está vinculada, el QR aparece en esa ventana y la tool devuelve un mensaje que el asistente transmite al usuario.
- Una vez vinculado, reintentar la tool funciona normalmente.
- Llamadas concurrentes a tools no arrancan dos daemons.
- Si ya hay un daemon corriendo (sesión previa, arranque manual), se reusa vía `/status` en lugar de duplicar.
- Al cerrar el MCP (normal o forzado), el daemon muere.
