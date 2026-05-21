package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"twins/internal/api"
	"twins/internal/core"
)

func main() {
	addr := os.Getenv("TWINS_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	env := strings.ToLower(strings.TrimSpace(os.Getenv("TWINS_ENV")))
	dataPath := strings.TrimSpace(os.Getenv("TWINS_DATA_PATH"))
	if env == "production" && dataPath == "" {
		log.Fatal("TWINS_DATA_PATH is required when TWINS_ENV=production")
	}

	var store *core.MemoryStore
	if dataPath != "" {
		persistentStore, err := core.NewPersistentStore(dataPath)
		if err != nil {
			log.Fatalf("open persistent store: %v", err)
		}
		store = persistentStore
		log.Printf("twins storage: file snapshot at %s", dataPath)
	} else {
		store = core.NewMemoryStore()
		log.Printf("twins storage: memory")
	}

	handler := api.NewServerWithOptions(store, api.ServerOptions{
		BusinessCreationToken: os.Getenv("TWINS_BUSINESS_CREATION_TOKEN"),
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("twins api listening on %s", addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("shutdown twins api: %v", err)
		}
		log.Print("twins api stopped")
	}
}
