package publisher

import "uuid"

type AvatarEventType int

const (
	AvatarEventUnknown AvatarEventType = iota
	AvatarEventCreateThumbnails
	AvatarEventDelete
)

type AvatarEvent struct {
	Type     AvatarEventType
	AvatarID uuid.UUID
}
