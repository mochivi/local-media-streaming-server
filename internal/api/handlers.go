package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

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

// Organize internal files by Type hierarchy: video files contain subtitle files
type MediaFileResponse struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Subtitles bool   `json:"subtitles"`
}

func (h *LibraryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	files := h.storage.Files()

	// Just a quick way to package subtitles, files with the same basename
	// avoids needing a database to tie files to subtitles
	// limits to only one subtitle file per video
	resp := make([]MediaFileResponse, 0, len(files))
	for _, media := range files {
		if slices.Contains(core.VideoExt, media.Type) {
			resp = append(resp, MediaFileResponse{
				Name:      media.BaseName(),
				Type:      media.Type,
				Subtitles: false,
			})
		}
	}
	for i := range resp {
		for _, media := range files {
			if media.Type != ".srt" {
				continue
			}
			if media.BaseName() == resp[i].Name {
				resp[i].Subtitles = true
				break
			}

		}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"count": len(resp),
		"data":  resp,
	})
}

type StreamHandler struct {
	storage core.FileStorage
}

func NewStreamHandler(storage core.FileStorage) *StreamHandler {
	return &StreamHandler{storage: storage}
}

func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filename, err := h.validateRequest(r)
	if err != nil {
		slog.Error("validation_error", "error", err)
		writeJSONError(w, "Parameter filename is required", http.StatusBadRequest)
		return
	}

	media, ok := h.storage.FindByName(filename)
	if !ok {
		slog.Error("file_not_found", "error", err)
		writeJSONError(w, "No file matches provided filename", http.StatusNotFound)
		return
	}

	// parse requested range
	start, end, err := h.parseRange(r.Header.Get("Range"), media.Size)
	if err != nil {
		slog.Error("invalid_range", "error", err)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", media.Size))
		writeJSONError(w, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	isRangeRequest := true
	if start == 0 && end == (media.Size-1) {
		isRangeRequest = false
	}

	if err := h.serveContent(w, media, start, end, isRangeRequest); err != nil {
		slog.Error("failed_serve_content", "error", err)
		writeJSONError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *StreamHandler) validateRequest(r *http.Request) (string, error) {
	filename := r.PathValue("filename")
	if filename == "" {
		return "", errors.New("filename param is required")
	}

	return filename, nil
}

func (h *StreamHandler) parseRange(header string, filesize int64) (int64, int64, error) {
	if header == "" {
		return 0, filesize - 1, nil // full file request
	}

	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, errors.New("invalid range unit")
	}

	parts := strings.SplitN(strings.TrimPrefix(header, "bytes="), "-", 2)
	if len(parts) != 2 {
		return 0, 0, errors.New("malformed range")
	}

	var start int64
	var end int64
	if parts[0] == "" {
		// suffix range: bytes=-500
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("malformed range: %w", err)
		}
		start = filesize - suffix
		end = filesize - 1
	} else if parts[1] == "" {
		// open ended: bytes=500-
		var err error
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("malformed range: %w", err)
		}
		end = filesize - 1
	} else {
		// bytes=500-999 --> ["500", "999"]
		var err error
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("malformed range: %w", err)
		}
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("malformed range: %w", err)
		}
	}

	if start < 0 || end < start || end >= filesize {
		return 0, 0, errors.New("range out of bounds")
	}

	return start, end, nil
}

func (h *StreamHandler) serveContent(w http.ResponseWriter, media core.MediaFile, start, end int64, isRangeRequest bool) error {
	rs, err := core.OpenSeekMedia(h.storage.FileSystem(), media, start)
	if err != nil {
		return err
	}
	defer rs.Close()

	contentLength := end - start + 1
	w.Header().Set("Content-Type", media.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.Header().Set("Accept-Ranges", "bytes")

	if isRangeRequest {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, media.Size))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	// stream file
	_, err = io.CopyN(w, rs, contentLength)
	if err != nil {
		// cannot return an error after WriteHeader
		slog.Error("stream_interrupted", "file", media.Name, "error", err)
	}

	return nil
}
