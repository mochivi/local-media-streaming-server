package core

import "time"

// Figure out what we wanna do with this
type MediaFile struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Type     string    `json:"type"`
	ModTime  time.Time `json:"modtime"`
	MimeType string    `json:"mimetype"`
}
