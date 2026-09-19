package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioConfig struct {
	Endpoint string
	User     string
	Password string
}

type Minio struct {
	client *minio.Client
}

func NewMinio(ctx context.Context, config MinioConfig) (*Minio, error) {
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(config.User, config.Password, ""),
	})
	if err != nil {
		return nil, fmt.Errorf("new client: %w", err)
	}

	ok, err := client.BucketExists(ctx, "avatars")
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}

	if !ok {
		if err := client.MakeBucket(ctx, "avatars", minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	return &Minio{
		client: client,
	}, nil
}

func (m *Minio) Ping(ctx context.Context) error {
	url := m.client.EndpointURL().JoinPath("minio", "health", "live").String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ErrNotHealthy
	}

	return nil
}

func (m *Minio) PutAvatar(ctx context.Context, input PutAvatarInput) (PutAvatarOutput, error) {
	info, err := m.client.PutObject(ctx, "avatars", input.Filename, input.Reader, input.Size, minio.PutObjectOptions{
		ContentType: input.ContentType,
	})
	if err != nil {
		return PutAvatarOutput{}, fmt.Errorf("put object: %w", err)
	}

	return PutAvatarOutput{
		Key: info.Key,
	}, nil
}

func (m *Minio) GetAvatar(ctx context.Context, key string) (GetAvatarOutput, error) {
	object, err := m.client.GetObject(ctx, "avatars", key, minio.GetObjectOptions{})
	if err != nil {
		return GetAvatarOutput{}, fmt.Errorf("get object: %w", err)
	}
	defer object.Close()

	bytes, err := io.ReadAll(object)
	if err != nil {
		return GetAvatarOutput{}, fmt.Errorf("read all: %w", err)
	}

	return GetAvatarOutput{
		Bytes: bytes,
	}, nil
}

func (m *Minio) DeleteAvatar(ctx context.Context, key string) error {
	if err := m.client.RemoveObject(ctx, "avatars", key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove object: %w", err)
	}

	return nil
}
