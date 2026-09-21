package services

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"
	"uuid"

	"github.com/HugoSmits86/nativewebp"
	"golang.org/x/image/draw"

	"github.com/sudeeya/gophprofile/internal/repository"
	"github.com/sudeeya/gophprofile/internal/storage"
)

var _thumbnailSizes = []struct{ width, height int }{
	{width: 100, height: 100},
	{width: 300, height: 300},
}

type AvatarThumbnailService struct {
	repo    AvatarRepository
	storage AvatarStorage
}

func NewAvatarThumbnailService(repo AvatarRepository, storage AvatarStorage) *AvatarThumbnailService {
	return &AvatarThumbnailService{
		repo:    repo,
		storage: storage,
	}
}

func (s *AvatarThumbnailService) GenerateThumbnail(ctx context.Context, id uuid.UUID) error {
	repoOutput, err := s.repo.GetAvatar(ctx, id)
	if err != nil {
		return fmt.Errorf("get avatar: %w", err)
	}

	repoMetadataOutput, err := s.repo.GetAvatarMetadata(ctx, id)
	if err != nil {
		return fmt.Errorf("get metadata: %w", err)
	}

	storageOutput, err := s.storage.GetAvatar(ctx, repoOutput.S3Key)
	if err != nil {
		return fmt.Errorf("get avatar: %w", err)
	}

	img, format, err := decodeImage(storageOutput.Bytes)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	for _, size := range _thumbnailSizes {
		var (
			scaled = scaleImage(img, size.width, size.height)
			buf    bytes.Buffer
		)

		if err := encodeImage(&buf, scaled, format); err != nil {
			return fmt.Errorf("encode image: %w", err)
		}

		storageOutput, err := s.storage.PutAvatar(ctx, storage.PutAvatarInput{
			Filename:    thumbnailFilename(repoMetadataOutput.Filename, size.width, size.height),
			ContentType: repoMetadataOutput.MimeType,
			Size:        int64(buf.Len()),
			Reader:      &buf,
		})
		if err != nil {
			return fmt.Errorf("put avatar: %w", err)
		}

		if err := s.repo.AddThumbnailKey(ctx, repository.AddThumbnailKeyInput{
			AvatarID: id,
			S3Key:    storageOutput.Key,
		}); err != nil {
			return fmt.Errorf("add thumbnail key: %w", err)
		}
	}

	return nil
}

func decodeImage(b []byte) (image.Image, string, error) {
	_, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, "", err
	}

	return image.Decode(bytes.NewReader(b))
}

func scaleImage(src image.Image, width, heigth int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, heigth))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst
}

func encodeImage(w io.Writer, img image.Image, format string) error {
	switch format {
	case "jpeg":
		if err := jpeg.Encode(w, img, nil); err != nil {
			return fmt.Errorf("encode jpeg: %w", err)
		}
	case "png":
		if err := png.Encode(w, img); err != nil {
			return fmt.Errorf("encode png: %w", err)
		}
	case "webp":
		if err := nativewebp.Encode(w, img, nil); err != nil {
			return fmt.Errorf("encode webp: %w", err)
		}
	default:
		return ErrFormatNotSupported
	}

	return nil
}

func thumbnailFilename(avatarFilename string, width, height int) string {
	ext := filepath.Ext(avatarFilename)
	name := strings.TrimSuffix(avatarFilename, ext)
	return fmt.Sprintf("%s_%dx%d%s", name, width, height, ext)
}

func (s *AvatarThumbnailService) DeleteAvatar(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := s.storage.DeleteAvatar(ctx, key); err != nil {
			return fmt.Errorf("delete avatar: %w", err)
		}
	}

	return nil
}
