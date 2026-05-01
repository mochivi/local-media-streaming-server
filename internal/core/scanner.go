package core

import (
	"context"
	"io/fs"
	"log/slog"
	"slices"
	"sync"
	"time"
)

type FileStorage interface {
	Files() []MediaFile
	FindByName(name string) (MediaFile, bool)
	Open(name string) (fs.File, error)
}

type ScannerConfig struct {
	scanPeriodSeconds time.Duration
}

type ScannerFileStorage struct {
	ctx context.Context

	config ScannerConfig

	fs      fs.FS
	mu      sync.RWMutex
	fileSet map[string]bool
	files   []MediaFile
}

func NewScannerFileStorage(ctx context.Context, fs fs.FS) FileStorage {
	scanner := &ScannerFileStorage{
		ctx: ctx,
		config: ScannerConfig{
			scanPeriodSeconds: 10 * time.Second,
		},
		fs: fs,
	}

	go func() {
		scanner.start()
	}()

	return scanner
}

func (s *ScannerFileStorage) Files() []MediaFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.files)
}

func (s *ScannerFileStorage) FindByName(name string) (MediaFile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, exists := s.fileSet[name]; !exists {
		return MediaFile{}, false
	}
	for _, file := range s.files {
		if file.Name == name {
			return file, true
		}
	}
	return MediaFile{}, false
}

func (s *ScannerFileStorage) Open(name string) (fs.File, error) { return nil, nil }

func (s *ScannerFileStorage) start() error {
	// Scan the first time at start up, then launch scanning loop
	if err := s.scan(); err != nil {
		return err
	}
	if err := s.scan_routine(s.config.scanPeriodSeconds); err != nil {
		return err
	}
	return nil
}

func (s *ScannerFileStorage) scan_routine(scanPeriod time.Duration) error {
	for {
		select {
		case <-time.Tick(scanPeriod):
			if err := s.scan(); err != nil {
				return err
			}
		case <-s.ctx.Done():
			return s.ctx.Err()
		}
	}
}

func (s *ScannerFileStorage) scan() error {
	fileSet := make(map[string]bool, 0)
	files := make([]MediaFile, 0)
	if err := fs.WalkDir(s.fs, ".", func(path string, d fs.DirEntry, err error) error {
		if d == nil || d.IsDir() {
			return nil // resursive walk enabled
		}
		fileSet[d.Name()] = true
		files = append(files, MediaFile{
			Name:     d.Name(),
			MimeType: "", // extract details from the file
		})
		return nil
	}); err != nil {
		slog.Error("scanner_walk_error", "error", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.files = files
	s.fileSet = fileSet

	return nil
}
