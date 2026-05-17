package core

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"
)

var VideoExt []string = []string{".mp4", ".mkv"}
var SubtitleExt []string = []string{".srt"}

type MediaFile struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Type     string    `json:"type"`
	ModTime  time.Time `json:"modtime"`
	MimeType string    `json:"mimetype"`
}

func (m MediaFile) BaseName() string {
	return strings.TrimSuffix(m.Name, m.Type)
}

func OpenMedia(fileSystem fs.FS, media MediaFile) (fs.File, error) {
	f, err := fileSystem.Open(media.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return f, nil
}

func OpenSeekMedia(fileSystem fs.FS, media MediaFile, start int64) (io.ReadSeekCloser, error) {
	f, err := fileSystem.Open(media.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	rs, ok := f.(io.ReadSeekCloser)
	if !ok {
		return nil, errors.New("file is not seekable")
	}
	_, err = rs.Seek(start, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}
	return rs, nil
}
