package main

import (
	"log"
	"net/http"
	"os"

	"twins/internal/api"
	"twins/internal/core"
)

func main() {
	addr := os.Getenv("TWINS_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	store := core.NewMemoryStore()
	handler := api.NewServer(store)

	log.Printf("twins api listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
