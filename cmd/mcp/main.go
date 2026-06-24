package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"whatsapp-mcp/internal/config"
	"whatsapp-mcp/internal/ipc"
	"whatsapp-mcp/internal/launcher"
	"whatsapp-mcp/internal/store"
	"whatsapp-mcp/internal/tools"
)

func main() {
	configPath := "config.json"
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	st, err := store.Open(cfg.MessagesDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	daemon := &ipc.Client{BaseURL: cfg.BaseURL(), Token: cfg.IPCToken, HTTP: http.DefaultClient}
	workDir := configWorkDir(configPath)
	daemonPath := resolveDaemonPath(cfg, workDir)
	l := &launcher.Launcher{DaemonPath: daemonPath, WorkDir: workDir, Client: daemon}
	defer l.Close()
	t := &tools.Tools{Store: st, Daemon: daemon, Launcher: l}

	server := mcp.NewServer(&mcp.Implementation{Name: "whatsapp-mcp", Version: "v0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "get_connection_status", Description: "Estado de la conexión a WhatsApp (conectado, necesita QR, número vinculado)"}, t.GetConnectionStatus)
	mcp.AddTool(server, &mcp.Tool{Name: "search_contacts", Description: "Buscar contactos/chats por nombre o teléfono; devuelve candidatos con su JID"}, t.SearchContacts)
	mcp.AddTool(server, &mcp.Tool{Name: "sync_contacts", Description: "Sincronizar nombres desde la libreta de contactos de WhatsApp y aplicarlos a chats sin nombre"}, t.SyncContacts)
	mcp.AddTool(server, &mcp.Tool{Name: "list_chats", Description: "Listar chats recientes con vista previa del último mensaje"}, t.ListChats)
	mcp.AddTool(server, &mcp.Tool{Name: "get_messages", Description: "Traer mensajes de un chat (por nombre o JID). Si el nombre es ambiguo devuelve candidatos"}, t.GetMessages)
	mcp.AddTool(server, &mcp.Tool{Name: "list_media", Description: "Listar adjuntos (imagen/audio/video/documento/sticker) disponibles en un chat"}, t.ListMedia)
	mcp.AddTool(server, &mcp.Tool{Name: "download_media", Description: "Descargar adjuntos de un chat a una carpeta destino; devuelve las rutas guardadas"}, t.DownloadMedia)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func configWorkDir(configPath string) string {
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return "."
	}
	return filepath.Dir(abs)
}

func resolveDaemonPath(cfg config.Config, workDir string) string {
	if cfg.DaemonPath != "" {
		if filepath.IsAbs(cfg.DaemonPath) {
			return cfg.DaemonPath
		}
		return filepath.Join(workDir, cfg.DaemonPath)
	}
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join(workDir, "whatsapp-daemon.exe")
	}
	return filepath.Join(filepath.Dir(exe), "whatsapp-daemon.exe")
}
