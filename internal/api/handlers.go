package api

import (
	"encoding/json"
	"net/http"

	"github.com/mochivi/local-media-streaming-server/internal/core"
)

type LibraryHandler struct {
	storage core.FileStorage
}

func NewLibraryHandler(storage core.FileStorage) *LibraryHandler {
	return &LibraryHandler{
		storage: storage,
	}
}

func (h *LibraryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// auth --> middleware later on

	files := h.storage.Files()
	json.NewEncoder(w).Encode(map[string]any{
		"count": len(files),
		"data":  files,
	})
}
