package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]any{"error": message}); err != nil {
		slog.Error("json_encode_error", "error", err)
	}
}
