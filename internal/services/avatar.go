package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"uuid"

	"github.com/sudeeya/gophprofile/internal/broker"
	"github.com/sudeeya/gophprofile/internal/domain"
	"github.com/sudeeya/gophprofile/internal/repository"
	"github.com/sudeeya/gophprofile/internal/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const MaxAvatarSize = 10 * 1024 * 1024

var supportedAvatarFormats = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

type AvatarService struct {
	repo      AvatarRepository
	storage   AvatarStorage
	publisher AvatarEventPublisher
	tracer    trace.Tracer
}

type AvatarRepository interface {
	CreateAvatar(ctx context.Context, input repository.CreateAvatarInput) (repository.CreateAvatarOutput, error)
	GetAvatar(ctx context.Context, id uuid.UUID) (repository.GetAvatarOutput, error)
	GetAvatarMetadata(ctx context.Context, id uuid.UUID) (repository.GetAvatarMetadataOutput, error)
	DeleteAvatar(ctx context.Context, id uuid.UUID) error
	AddThumbnailKey(ctx context.Context, input repository.AddThumbnailKeyInput) error
}

type AvatarStorage interface {
	PutAvatar(ctx context.Context, input storage.PutAvatarInput) (storage.PutAvatarOutput, error)
	GetAvatar(ctx context.Context, key string) (storage.GetAvatarOutput, error)
	DeleteAvatar(ctx context.Context, key string) error
}

type AvatarEventPublisher interface {
	PublishAvatarUploadEvent(ctx context.Context, event broker.AvatarUploadEvent) error
	PublishAvatarDeleteEvent(ctx context.Context, event broker.AvatarDeleteEvent) error
}

func NewAvatarService(repo AvatarRepository, storage AvatarStorage, publisher AvatarEventPublisher) *AvatarService {
	tracer := otel.Tracer("github.com/sudeeya/gophprofile/internal/services")

	return &AvatarService{
		repo:      repo,
		storage:   storage,
		publisher: publisher,
		tracer:    tracer,
	}
}

type UploadAvatarInput struct {
	UserID   string
	Filename string
	Reader   io.Reader
}

func (s *AvatarService) UploadAvatar(ctx context.Context, input UploadAvatarInput) (domain.Avatar, error) {
	ctx, span := s.tracer.Start(ctx, "upload avatar",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("file.name", input.Filename),
			attribute.String("user.id", input.UserID),
		),
	)
	defer span.End()

	var buf bytes.Buffer
	size, err := io.Copy(&buf, io.LimitReader(input.Reader, MaxAvatarSize+1))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return domain.Avatar{}, fmt.Errorf("copy: %w", err)
	}

	span.SetAttributes(attribute.Int64("file.size", size))
	if size > MaxAvatarSize {
		span.SetStatus(codes.Error, ErrAvatarTooLarge.Error())
		return domain.Avatar{}, ErrAvatarTooLarge
	}

	contentType := http.DetectContentType(buf.Bytes())
	span.SetAttributes(attribute.String("file.mime_type", contentType))
	if _, ok := supportedAvatarFormats[contentType]; !ok {
		span.SetStatus(codes.Error, ErrFormatNotSupported.Error())
		return domain.Avatar{}, ErrFormatNotSupported
	}

	storageOutput, err := s.storage.PutAvatar(ctx, storage.PutAvatarInput{
		Filename:    input.Filename,
		ContentType: contentType,
		Size:        size,
		Reader:      &buf,
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return domain.Avatar{}, fmt.Errorf("put avatar: %w", err)
	}

	span.SetAttributes(attribute.String("aws.s3.key", storageOutput.Key))

	repoOutput, err := s.repo.CreateAvatar(ctx, repository.CreateAvatarInput{
		UserID:   input.UserID,
		Filename: input.Filename,
		MimeType: contentType,
		S3Key:    storageOutput.Key,
		Size:     size,
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		_ = s.storage.DeleteAvatar(ctx, storageOutput.Key)
		return domain.Avatar{}, fmt.Errorf("create avatar: %w", err)
	}

	span.SetAttributes(attribute.String("avatar.id", repoOutput.ID.String()))

	_ = s.publisher.PublishAvatarUploadEvent(ctx, broker.AvatarUploadEvent{
		ID:    repoOutput.ID,
		S3Key: storageOutput.Key,
	})

	return domain.Avatar{
		Metadata: domain.Metadata{
			ID:        repoOutput.ID,
			UserID:    input.UserID,
			CreatedAt: repoOutput.CreatedAt,
		},
	}, nil
}

func (s *AvatarService) GetAvatar(ctx context.Context, id uuid.UUID) (domain.Avatar, error) {
	ctx, span := s.tracer.Start(ctx, "get avatar",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("avatar.id", id.String()),
		),
	)
	defer span.End()

	repoOutput, err := s.repo.GetAvatar(ctx, id)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return domain.Avatar{}, ErrAvatarNotFound
	case err != nil:
		span.SetStatus(codes.Error, err.Error())
		return domain.Avatar{}, fmt.Errorf("get avatar: %w", err)
	}

	storageOutput, err := s.storage.GetAvatar(ctx, repoOutput.S3Key)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return domain.Avatar{}, fmt.Errorf("get avatar: %w", err)
	}

	span.SetAttributes(
		attribute.String("aws.s3.key", repoOutput.S3Key),
		attribute.String("file.mime_type", repoOutput.MimeType),
	)

	return domain.Avatar{
		Bytes: storageOutput.Bytes,
		Metadata: domain.Metadata{
			MimeType: repoOutput.MimeType,
		},
	}, nil
}

func (s *AvatarService) GetAvatarMetadata(ctx context.Context, id uuid.UUID) (domain.Metadata, error) {
	ctx, span := s.tracer.Start(ctx, "get avatar metadata",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("avatar.id", id.String()),
		),
	)
	defer span.End()

	repoOutput, err := s.repo.GetAvatarMetadata(ctx, id)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return domain.Metadata{}, ErrMetadataNotFound
	case err != nil:
		span.SetStatus(codes.Error, err.Error())
		return domain.Metadata{}, fmt.Errorf("get metadata: %w", err)
	}

	span.SetAttributes(
		attribute.String("user.id", repoOutput.UserID),
		attribute.String("file.name", repoOutput.Filename),
		attribute.String("file.mime_type", repoOutput.MimeType),
		attribute.Int("file.size", repoOutput.Size),
	)

	return domain.Metadata{
		ID:        repoOutput.ID,
		UserID:    repoOutput.UserID,
		Filename:  repoOutput.Filename,
		MimeType:  repoOutput.MimeType,
		Size:      repoOutput.Size,
		CreatedAt: repoOutput.CreatedAt,
		UpdatedAt: repoOutput.UpdatedAt,
	}, nil
}

type DeleteAvatarInput struct {
	ID     uuid.UUID
	UserID string
}

func (s *AvatarService) DeleteAvatar(ctx context.Context, input DeleteAvatarInput) error {
	ctx, span := s.tracer.Start(ctx, "delete avatar",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("avatar.id", input.ID.String()),
			attribute.String("user.id", input.UserID),
		),
	)
	defer span.End()

	metadata, err := s.repo.GetAvatarMetadata(ctx, input.ID)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("get metadata: %w", err)
	}

	if input.UserID != metadata.UserID {
		return ErrForbidden
	}

	if err := s.repo.DeleteAvatar(ctx, input.ID); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("delete avatar: %w", err)
	}

	if err := s.publisher.PublishAvatarDeleteEvent(ctx, broker.AvatarDeleteEvent{
		ID:     input.ID,
		S3Keys: append(metadata.ThumbnailS3Keys, metadata.S3Key),
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("publish event: %w", err)
	}

	return nil
}
