package ipc

type Status struct {
	Connected bool   `json:"connected"`
	NeedsQR   bool   `json:"needs_qr"`
	JID       string `json:"jid"`
}

type DownloadRequest struct {
	Chat       string   `json:"chat"`
	DestFolder string   `json:"dest_folder"`
	Types      []string `json:"types"`
	Limit      int      `json:"limit"`
}

type SavedFile struct {
	Path     string `json:"path"`
	Mimetype string `json:"mimetype"`
	Size     int64  `json:"size"`
}

type DownloadResult struct {
	Files  []SavedFile `json:"files"`
	Errors []string    `json:"errors"`
}

type SyncContactsResult struct {
	Contacts int `json:"contacts"`
	Chats    int `json:"chats"`
}
