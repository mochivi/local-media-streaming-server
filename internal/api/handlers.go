package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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

func (h *LibraryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	files := h.storage.Files()
	json.NewEncoder(w).Encode(map[string]any{
		"count": len(files),
		"data":  files,
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

	if err := h.serveContent(w, media, start, end); err != nil {
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

func (h *StreamHandler) serveContent(w http.ResponseWriter, media core.MediaFile, start, end int64) error {
	f, err := h.storage.Open(media.Name)
	if err != nil {
		return errors.New("failed to open file")
	}
	defer f.Close()

	rs, ok := f.(io.ReadSeeker)
	if !ok {
		return errors.New("file is not seekable")
	}

	_, err = rs.Seek(start, io.SeekStart)
	if err != nil {
		return errors.New("failed to seek file")
	}

	contentLength := end - start + 1

	w.Header().Set("Content-Type", media.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, media.Size))

	if start == 0 && end == (media.Size-1) {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusPartialContent)
	}

	// stream file
	_, err = io.CopyN(w, rs, contentLength)
	if err != nil {
		// cannot return an error after WriteHeader
		slog.Error("stream_interrupted", "file", media.Name, "error", err)
	}

	return nil
}
