package domain

import (
	"time"
	"uuid"
)

type Avatar struct {
	Bytes    []byte   `json:"-"`
	Metadata Metadata `json:"metadata"`
}

type Metadata struct {
	ID        uuid.UUID `json:"id"`
	UserID    string    `json:"user_id"`
	Filename  string    `json:"file_name"`
	MimeType  string    `json:"mime_type"`
	Size      int       `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
