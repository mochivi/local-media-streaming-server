package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mochivi/local-media-streaming-server/api"
	"github.com/mochivi/local-media-streaming-server/core"
)

func main() {
	// dependencies
	ctx := context.Background()
	cwd, _ := os.Getwd()
	root := filepath.Join(cwd, "/data")
	fs := os.DirFS(root)

	scanner := core.NewFileScanner(ctx, fs, root)
	libraryHandler := api.NewLibraryHandler(ctx, scanner)

	go func() {
		libraryHandler.Start()
	}()

	// http server
	mux := http.NewServeMux()
	mux.Handle("/api/library", libraryHandler)
	mux.HandleFunc("/api/stream/{filename}", func(w http.ResponseWriter, r *http.Request) {
		log.Print("streaming not implemented")
	})
	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// serve
	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("Server shutdown with an error: %v", err)

	}
	log.Printf("Server shutdown normally")
}
