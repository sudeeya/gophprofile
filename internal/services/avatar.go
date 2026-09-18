package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"uuid"

	"github.com/sudeeya/gophprofile/internal/domain"
	"github.com/sudeeya/gophprofile/internal/publisher"
	"github.com/sudeeya/gophprofile/internal/repository"
	"github.com/sudeeya/gophprofile/internal/storage"
)

const MaxAvatarSize = 10 * 1024 * 1024

var _supportedAvatarFormats = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

type AvatarService struct {
	repo      AvatarRepository
	storage   AvatarStorage
	publisher AvatarEventPublisher
}

type AvatarRepository interface {
	CreateAvatar(ctx context.Context, input repository.CreateAvatarInput) (repository.CreateAvatarOutput, error)
	GetAvatar(ctx context.Context, id uuid.UUID) (repository.GetAvatarOutput, error)
	GetAvatarMetadata(ctx context.Context, id uuid.UUID) (repository.GetAvatarMetadataOutput, error)
	DeleteAvatar(ctx context.Context, id uuid.UUID) error
}

type AvatarStorage interface {
	PutAvatar(ctx context.Context, input storage.PutAvatarInput) (storage.PutAvatarOutput, error)
	GetAvatar(ctx context.Context, key string) (storage.GetAvatarOutput, error)
}

type AvatarEventPublisher interface {
	Publish(ctx context.Context, event publisher.AvatarEvent) error
}

func NewAvatarService(repo AvatarRepository, storage AvatarStorage, publisher AvatarEventPublisher) *AvatarService {
	return &AvatarService{
		repo:      repo,
		storage:   storage,
		publisher: publisher,
	}
}

type UploadAvatarInput struct {
	UserID   string
	Filename string
	Reader   io.Reader
}

func (s *AvatarService) UploadAvatar(ctx context.Context, input UploadAvatarInput) (domain.Avatar, error) {
	var buf bytes.Buffer
	size, err := io.Copy(&buf, io.LimitReader(input.Reader, MaxAvatarSize+1))
	if err != nil {
		return domain.Avatar{}, fmt.Errorf("copy: %w", err)
	}

	if size > MaxAvatarSize {
		return domain.Avatar{}, ErrAvatarTooLarge
	}

	contentType := http.DetectContentType(buf.Bytes())
	if _, ok := _supportedAvatarFormats[contentType]; !ok {
		return domain.Avatar{}, ErrFormatNotSupported
	}

	storageOutput, err := s.storage.PutAvatar(ctx, storage.PutAvatarInput{
		Filename:    input.Filename,
		ContentType: contentType,
		Size:        size,
		Reader:      &buf,
	})
	if err != nil {
		return domain.Avatar{}, err
	}

	repoOutput, err := s.repo.CreateAvatar(ctx, repository.CreateAvatarInput{
		UserID:   input.UserID,
		Filename: input.Filename,
		MimeType: contentType,
		S3Key:    storageOutput.Key,
		Size:     size,
	})
	if err != nil {
		return domain.Avatar{}, err
	}

	if err := s.publisher.Publish(ctx, publisher.AvatarEvent{
		Type:     publisher.AvatarEventCreateThumbnails,
		AvatarID: repoOutput.ID,
	}); err != nil {
		return domain.Avatar{}, err
	}

	return domain.Avatar{
		Metadata: domain.Metadata{
			ID:        repoOutput.ID,
			UserID:    input.UserID,
			CreatedAt: repoOutput.CreatedAt,
		},
	}, nil
}

func (s *AvatarService) GetAvatar(ctx context.Context, id uuid.UUID) (domain.Avatar, error) {
	repoOutput, err := s.repo.GetAvatar(ctx, id)
	if err != nil {
		return domain.Avatar{}, err
	}

	storageOutput, err := s.storage.GetAvatar(ctx, repoOutput.S3Key)
	if err != nil {
		return domain.Avatar{}, err
	}

	return domain.Avatar{
		Bytes: storageOutput.Bytes,
		Metadata: domain.Metadata{
			MimeType: repoOutput.MimeType,
		},
	}, nil
}

func (s *AvatarService) GetAvatarMetadata(ctx context.Context, id uuid.UUID) (domain.Metadata, error) {
	repoOutput, err := s.repo.GetAvatarMetadata(ctx, id)
	if err != nil {
		return domain.Metadata{}, err
	}

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
	metadata, err := s.repo.GetAvatarMetadata(ctx, input.ID)
	if err != nil {
		return err
	}

	if input.UserID != metadata.UserID {
		return ErrForbidden
	}

	if err := s.repo.DeleteAvatar(ctx, input.ID); err != nil {
		return err
	}

	if err := s.publisher.Publish(ctx, publisher.AvatarEvent{
		Type:     publisher.AvatarEventDelete,
		AvatarID: input.ID,
	}); err != nil {
		return err
	}

	return nil
}
