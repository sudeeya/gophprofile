package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"uuid"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type MinioConfig struct {
	Endpoint string
	User     string
	Password string
}

type Minio struct {
	client *minio.Client
	tracer trace.Tracer
}

func NewMinio(ctx context.Context, config MinioConfig) (*Minio, error) {
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(config.User, config.Password, ""),
	})
	if err != nil {
		return nil, fmt.Errorf("new client: %w", err)
	}

	ok, err := client.BucketExists(ctx, BucketAvatar)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}

	if !ok {
		if err := client.MakeBucket(ctx, BucketAvatar, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	tracer := otel.Tracer("github.com/sudeeya/gophprofile/internal/storage")

	return &Minio{
		client: client,
		tracer: tracer,
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
	ctx, span := m.tracer.Start(ctx, "PutObject avatars",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("rpc.system.name", "aws-api"),
			attribute.String("rpc.method", "PutObject"),
			attribute.String("aws.s3.bucket", BucketAvatar),
		),
	)
	defer span.End()

	info, err := m.client.PutObject(ctx, BucketAvatar, uuid.New().String(), input.Reader, input.Size, minio.PutObjectOptions{
		ContentType: input.ContentType,
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return PutAvatarOutput{}, fmt.Errorf("put object: %w", err)
	}

	span.SetAttributes(attribute.String("aws.s3.key", info.Key))

	return PutAvatarOutput{
		Key: info.Key,
	}, nil
}

func (m *Minio) GetAvatar(ctx context.Context, key string) (GetAvatarOutput, error) {
	ctx, span := m.tracer.Start(ctx, "GetObject avatars",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("rpc.system.name", "aws-api"),
			attribute.String("rpc.method", "GetObject"),
			attribute.String("aws.s3.bucket", BucketAvatar),
			attribute.String("aws.s3.key", key),
		),
	)
	defer span.End()

	object, err := m.client.GetObject(ctx, BucketAvatar, key, minio.GetObjectOptions{})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return GetAvatarOutput{}, fmt.Errorf("get object: %w", err)
	}
	defer object.Close()

	bytes, err := io.ReadAll(object)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return GetAvatarOutput{}, fmt.Errorf("read all: %w", err)
	}

	return GetAvatarOutput{
		Bytes: bytes,
	}, nil
}

func (m *Minio) DeleteAvatar(ctx context.Context, key string) error {
	ctx, span := m.tracer.Start(ctx, "DeleteObject avatars",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("rpc.system.name", "aws-api"),
			attribute.String("rpc.method", "DeleteObject"),
			attribute.String("aws.s3.bucket", BucketAvatar),
			attribute.String("aws.s3.key", key),
		),
	)
	defer span.End()

	if err := m.client.RemoveObject(ctx, BucketAvatar, key, minio.RemoveObjectOptions{}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("remove object: %w", err)
	}

	return nil
}
