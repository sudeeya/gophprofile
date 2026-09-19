package broker

import "uuid"

const (
	_exchange    = "avatars"
	_queueUpload = "avatars.upload"
	_queueDelete = "avatars.delete"
	_keyUpload   = "avatar.upload"
	_keyDelete   = "avatar.delete"
)

type AvatarUploadEvent struct {
	ID    uuid.UUID `json:"id"`
	S3Key string    `json:"s3_key"`
}

type AvatarDeleteEvent struct {
	ID     uuid.UUID `json:"id"`
	S3Keys []string  `json:"s3_keys"`
}
