package repository

import (
	"time"
	"uuid"
)

type CreateAvatarInput struct {
	UserID   string
	Filename string
	MimeType string
	S3Key    string
	Size     int64
}

type CreateAvatarOutput struct {
	ID        uuid.UUID
	CreatedAt time.Time
}

type GetAvatarOutput struct {
	MimeType string
	S3Key    string
}

type GetAvatarMetadataOutput struct {
	ID              uuid.UUID
	UserID          string
	Filename        string
	MimeType        string
	S3Key           string
	ThumbnailS3Keys []string
	Size            int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AddThumbnailKeyInput struct {
	AvatarID uuid.UUID
	S3Key    string
}
