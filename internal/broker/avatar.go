package broker

import (
	"fmt"
	"uuid"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeAvatar       = "avatars"
	QueueAvatarUpload    = "avatars.upload"
	QueueAvatarDelete    = "avatars.delete"
	EventKeyAvatarUpload = "avatar.upload"
	EventKeyAvatarDelete = "avatar.delete"
)

type AvatarUploadEvent struct {
	ID    uuid.UUID `json:"id"`
	S3Key string    `json:"s3_key"`
}

type AvatarDeleteEvent struct {
	ID     uuid.UUID `json:"id"`
	S3Keys []string  `json:"s3_keys"`
}

func declareTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(ExchangeAvatar, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}

	if _, err := ch.QueueDeclare(QueueAvatarUpload, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	if err := ch.QueueBind(QueueAvatarUpload, EventKeyAvatarUpload, ExchangeAvatar, false, nil); err != nil {
		return fmt.Errorf("bind queue: %w", err)
	}

	if _, err := ch.QueueDeclare(QueueAvatarDelete, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	if err := ch.QueueBind(QueueAvatarDelete, EventKeyAvatarDelete, ExchangeAvatar, false, nil); err != nil {
		return fmt.Errorf("bind queue: %w", err)
	}

	return nil
}
