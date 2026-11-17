package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"kvdb/internal/handler"
	"kvdb/internal/server"
	"kvdb/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	store := storage.NewMemoryStore()

	h, err := handler.New(store)
	if err != nil {
		log.Fatalf("create handler: %v", err)
	}

	srv, err := server.New(":9000", h)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}
	defer srv.Close()

	log.Println("KVDB UDP server listening on :9000")

	if err := srv.Serve(ctx); err != nil {
		log.Printf("server stopped with error: %v", err)
	}
}
