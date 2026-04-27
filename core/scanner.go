package core

import (
	"context"
	"fmt"
	"io/fs"
	"sync"
	"time"
)

type ScannerConfig struct {
	scanPeriodSeconds time.Duration
}

type FileScanner struct {
	ctx context.Context

	fs   fs.FS
	root string

	filesMu sync.RWMutex
	fileSet map[string]bool
	files   []MediaFile

	config ScannerConfig
}

func NewFileScanner(ctx context.Context, fs fs.FS, root string) *FileScanner {
	return &FileScanner{
		fs:   fs,
		root: root,
		ctx:  ctx,
		config: ScannerConfig{
			scanPeriodSeconds: 10 * time.Second,
		},
	}
}

func (s *FileScanner) Scan() error {
	fmt.Print("scanning root directory")

	fileSet := make(map[string]bool, 0)
	files := make([]MediaFile, 0)
	fs.WalkDir(s.fs, s.root, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil // resursive walk enabled
		}
		fileSet[d.Name()] = true
		files = append(files, MediaFile{
			Name:     d.Name(),
			MimeType: "", // extract details from the file
		})
		return nil
	})

	s.filesMu.Lock()
	s.fileSet = fileSet
	s.files = files
	s.filesMu.Unlock()

	return nil
}

func (s *FileScanner) GetFiles() []MediaFile {
	s.filesMu.RLock()
	defer s.filesMu.RUnlock()
	return s.files
}

func (s *FileScanner) Exists(name string) bool {
	s.filesMu.RLock()
	defer s.filesMu.RUnlock()
	if _, ok := s.fileSet[name]; !ok {
		return false
	}
	return true
}
