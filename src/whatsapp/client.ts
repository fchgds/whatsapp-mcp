import makeWASocket, {
  DisconnectReason,
  fetchLatestWaWebVersion,
  useMultiFileAuthState,
  type WASocket,
  type ConnectionState,
} from "@whiskeysockets/baileys"
import pino from "pino"
import { join } from "node:path"
import { mkdirSync, rmSync } from "node:fs"
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
let qrReject: ((err: Error) => void) | undefined
let reregisterCallback: (() => void) | undefined

let logger = pino({ level: "silent" })

export function setReregisterCallback(fn: () => void): void {
  reregisterCallback = fn
}

export async function initClient(dbPath: string, logLevel: string = "silent"): Promise<void> {
  authDir = join(dbPath, "auth")
  mkdirSync(authDir, { recursive: true })
  logger = pino({ level: logLevel }, pino.destination(2))
  await createSocket()
}

async function createSocket(): Promise<void> {
  const { state, saveCreds: sc } = await useMultiFileAuthState(authDir)
  saveCreds = sc
  const { version } = await fetchLatestWaWebVersion()
  const sock = makeWASocket({ auth: state, printQRInTerminal: false, logger, version })
  waState.socket = sock
  waState.status = "connecting"
  sock.ev.on("creds.update", () => saveCreds?.())
  sock.ev.on("connection.update", handleConnectionUpdate)
  // Re-registrar handlers de mensajes en el nuevo socket
  reregisterCallback?.()
}

function handleConnectionUpdate(update: Partial<ConnectionState>): void {
  const { connection, lastDisconnect, qr } = update
  if (qr) {
    waState.status = "waiting_qr"
    pendingQR = qr
    const ascii = qrToASCII(qr)
    qrResolve?.(ascii)
    qrResolve = undefined
    qrReject = undefined
  }
  if (connection === "open") {
    waState.status = "connected"
    pendingQR = undefined
  }
  if (connection === "close") {
    waState.status = "disconnected"
    const code = (lastDisconnect?.error as { output?: { statusCode?: number } })?.output?.statusCode
    if (code !== DisconnectReason.loggedOut) createSocket()
    else {
      // Credenciales inválidas/corruptas: limpiar auth y solicitar nuevo QR
      qrReject?.(new Error("Sesión cerrada en el teléfono"))
      qrResolve = undefined
      qrReject = undefined
      rmSync(authDir, { recursive: true, force: true })
      mkdirSync(authDir, { recursive: true })
      createSocket()
    }
  }
}

export async function connectQR(): Promise<string> {
  if (waState.status === "connected") throw new Error("Ya conectado")
  if (pendingQR) return qrToASCII(pendingQR)
  return new Promise<string>((resolve, reject) => {
    qrResolve = resolve
    qrReject = reject
    // Timeout de 60 segundos (los QR de WhatsApp expiran en ~20s)
    setTimeout(() => {
      if (qrResolve) {
        qrResolve = undefined
        qrReject = undefined
        reject(new Error("Timeout esperando QR (60s). Reintentá llamando connect_qr nuevamente."))
      }
    }, 60_000)
  })
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
