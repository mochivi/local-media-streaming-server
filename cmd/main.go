package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	mediastreaming "github.com/mochivi/local-media-streaming-server/internal"
	"github.com/mochivi/local-media-streaming-server/internal/api"
	"github.com/mochivi/local-media-streaming-server/internal/core"
)

func main() {
	// dependencies
	ctx := context.Background()
	cwd, _ := os.Getwd()
	root := filepath.Join(cwd, "data")
	fs := os.DirFS(root)

	// Init logger, use default logging with slog.Info() for now
	// no passing *slog.Logger around
	mediastreaming.InitLogger("info")

	fileStorage := core.NewScannerFileStorage(ctx, fs)
	libraryHandler := api.NewLibraryHandler(fileStorage)

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
	slog.Info("Starting server...")
	if err := s.ListenAndServe(); err != nil {
		slog.Error("Server shutdown with an error", "error", err)
		os.Exit(1)
	}
	slog.Info("Server shutdown normally")
}
