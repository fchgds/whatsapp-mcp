package model

import "time"

type Chat struct {
	JID             string
	Name            string
	Type            string // "individual" | "group"
	LastMessageText string
	LastMessageTS   time.Time
}

type Contact struct {
	JID   string
	Name  string
	Phone string
}

type MediaInfo struct {
	Mimetype string
	Filename string
	Size     int64
}

type Message struct {
	ID        string
	ChatJID   string
	SenderJID string
	Timestamp time.Time
	Type      string // text|image|audio|video|document|sticker
	Text      string
	Media     *MediaInfo // nil si no hay adjunto
	RawProto  []byte     // waE2E.Message serializado, sólo para mensajes con media
}
