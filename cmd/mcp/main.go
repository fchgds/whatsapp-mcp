package main

import (
	"context"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"whatsapp-mcp/internal/config"
	"whatsapp-mcp/internal/ipc"
	"whatsapp-mcp/internal/store"
	"whatsapp-mcp/internal/tools"
)

func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	st, err := store.Open(cfg.MessagesDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	daemon := &ipc.Client{BaseURL: cfg.BaseURL(), Token: cfg.IPCToken, HTTP: http.DefaultClient}
	t := &tools.Tools{Store: st, Daemon: daemon}

	server := mcp.NewServer(&mcp.Implementation{Name: "whatsapp-mcp", Version: "v0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "get_connection_status", Description: "Estado de la conexión a WhatsApp (conectado, necesita QR, número vinculado)"}, t.GetConnectionStatus)
	mcp.AddTool(server, &mcp.Tool{Name: "search_contacts", Description: "Buscar contactos/chats por nombre o teléfono; devuelve candidatos con su JID"}, t.SearchContacts)
	mcp.AddTool(server, &mcp.Tool{Name: "list_chats", Description: "Listar chats recientes con vista previa del último mensaje"}, t.ListChats)
	mcp.AddTool(server, &mcp.Tool{Name: "get_messages", Description: "Traer mensajes de un chat (por nombre o JID). Si el nombre es ambiguo devuelve candidatos"}, t.GetMessages)
	mcp.AddTool(server, &mcp.Tool{Name: "list_media", Description: "Listar adjuntos (imagen/audio/video/documento/sticker) disponibles en un chat"}, t.ListMedia)
	mcp.AddTool(server, &mcp.Tool{Name: "download_media", Description: "Descargar adjuntos de un chat a una carpeta destino; devuelve las rutas guardadas"}, t.DownloadMedia)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
