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

export interface Contact {
  jid: string
  lid: string
  name: string
  notify: string
  verifiedName: string
  imgUrl: string
  status: string
}

export interface GetMessagesOpts {
  chatJID: string
  limit?: number
  offset?: number
  fromTimestamp?: number
  toTimestamp?: number
}
