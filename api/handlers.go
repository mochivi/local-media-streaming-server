package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

type LibraryHandler struct {
	fs      fs.FS
	root    string
	fileSet map[string]bool
	files   []string

	filesMu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	// Scan configuration
	scanPeriodSeconds time.Duration
}

func NewLibraryHandler(ctx context.Context, fs fs.FS, root string) *LibraryHandler {
	ctx, cancel := context.WithCancel(ctx)
	return &LibraryHandler{
		fs:                fs,
		root:              root,
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
	h.filesMu.RLock()
	defer h.filesMu.RUnlock()
	json.NewEncoder(w).Encode(map[string]any{
		"count": len(h.files),
		"data":  h.files,
	})
}

func (h *LibraryHandler) scan(scanPeriod time.Duration) error {
	for {
		select {
		case <-time.Tick(scanPeriod):
			if err := h._scan(); err != nil {
				return err
			}
		case <-h.ctx.Done():
			return h.ctx.Err()
		}
	}
}

func (h *LibraryHandler) _scan() error {
	fmt.Print("scanning root directory")

	fileSet := make(map[string]bool, 0)
	files := make([]string, 0)
	fs.WalkDir(h.fs, h.root, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil // resursive walk enabled
		}
		fileSet[d.Name()] = true
		files = append(files, d.Name())
		return nil
	})

	h.filesMu.Lock()
	h.fileSet = fileSet
	h.filesMu.Unlock()

	return nil
}
