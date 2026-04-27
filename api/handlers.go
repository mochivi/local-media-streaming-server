package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/mochivi/local-media-streaming-server/core"
)

type LibraryHandler struct {
	scanner *core.FileScanner

	ctx    context.Context
	cancel context.CancelFunc

	// Scan configuration
	scanPeriodSeconds time.Duration
}

func NewLibraryHandler(ctx context.Context, scanner *core.FileScanner) *LibraryHandler {
	ctx, cancel := context.WithCancel(ctx)
	return &LibraryHandler{
		scanner:           scanner,
		ctx:               ctx,
		cancel:            cancel,
		scanPeriodSeconds: 10 * time.Second,
	}
}

func (h *LibraryHandler) Start() error {
	if err := h.scan(h.scanPeriodSeconds); err != nil {
		return err
	}
	return nil
}

func (h *LibraryHandler) Close() {
	h.cancel()
}

func (h *LibraryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// auth --> middleware later on
	files := h.scanner.GetFiles()
	json.NewEncoder(w).Encode(map[string]any{
		"count": len(files),
		"data":  files,
	})
}

func (h *LibraryHandler) scan(scanPeriod time.Duration) error {
	for {
		select {
		case <-time.Tick(scanPeriod):
			if err := h.scanner.Scan(); err != nil {
				return err
			}
		case <-h.ctx.Done():
			return h.ctx.Err()
		}
	}
}
