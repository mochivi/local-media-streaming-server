package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	mediastreaming "github.com/mochivi/local-media-streaming-server/internal"
	"github.com/mochivi/local-media-streaming-server/internal/api"
	"github.com/mochivi/local-media-streaming-server/internal/core"
)

func fromArgs(args []string) string {
	root := args[0]
	return root
}

func main() {
	var root string
	if len(os.Args) > 1 {
		root = fromArgs(os.Args[1:])
	}
	if root == "" {
		cwd, _ := os.Getwd()
		root = filepath.Join(cwd, "data")
	}
	fs := os.DirFS(root)

	// dependencies
	ctx := context.Background()

	// Init logger, use default logging with slog.Info() for now
	// no passing *slog.Logger around
	mediastreaming.InitLogger("info")

	fileStorage := core.NewScannerFileStorage(ctx, fs)
	libraryHandler := api.NewLibraryHandler(fileStorage)
	streamingHandler := api.NewStreamHandler(fileStorage)

	// http server
	mux := http.NewServeMux()
	mux.Handle("/api/library", libraryHandler)
	mux.Handle("/api/stream/{filename}", streamingHandler)

	handler := api.RecoveryMiddleware(
		api.MetricsLoggingMiddleware(mux),
	)

	s := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// serve
	slog.Info("server_starting")
	if err := s.ListenAndServe(); err != nil {
		slog.Error("server_shutdown_error", "error", err)
		os.Exit(1)
	}
	slog.Info("server_shutdown")
}
