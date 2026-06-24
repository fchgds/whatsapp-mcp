package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"whatsapp-mcp/internal/config"
	"whatsapp-mcp/internal/ingest"
	"whatsapp-mcp/internal/ipc"
	"whatsapp-mcp/internal/store"
	"whatsapp-mcp/internal/wa"
)

func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(cfg.MessagesDBPath())
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	h := &ingest.Handler{Store: st}
	client, err := wa.New(ctx, cfg.SessionDBPath(), h)
	if err != nil {
		log.Fatalf("whatsmeow: %v", err)
	}
	if err := client.Start(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}

	go func() {
		names, err := client.SyncContacts(ctx)
		if err != nil {
			log.Printf("sync contacts: %v", err)
			return
		}
		contacts, chats, err := st.SyncContactNames(names)
		if err != nil {
			log.Printf("sync contact names: %v", err)
		} else {
			log.Printf("sync contacts: %d contactos, %d chats actualizados", contacts, chats)
		}
	}()

	backend := &wa.Backend{Store: st, WA: client}
	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", cfg.IPCPort), Handler: ipc.NewServer(cfg.IPCToken, backend)}
	go func() {
		log.Printf("IPC escuchando en %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ipc: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("apagando…")
	srv.Close()
}
