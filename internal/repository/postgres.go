package repository

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"uuid"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DB       string
}

type Postgres struct {
	pool    *pgxpool.Pool
	builder squirrel.StatementBuilderType
	tracer  trace.Tracer
}

func NewPostgres(ctx context.Context, config PostgresConfig) (*Postgres, error) {
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(config.User, config.Password),
		Host:   net.JoinHostPort(config.Host, config.Port),
		Path:   config.DB,
	}

	pool, err := pgxpool.New(ctx, dsn.String())
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	builder := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

	tracer := otel.Tracer("github.com/sudeeya/gophprofile/internal/repository")

	return &Postgres{
		pool:    pool,
		builder: builder,
		tracer:  tracer,
	}, nil
}

func (p *Postgres) Close() error {
	p.pool.Close()
	return nil
}

func (p *Postgres) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *Postgres) CreateAvatar(ctx context.Context, input CreateAvatarInput) (CreateAvatarOutput, error) {
	ctx, span := p.tracer.Start(ctx, "INSERT avatars",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system.name", "postgresql"),
			attribute.String("db.collection.name", "avatars"),
			attribute.String("db.operation.name", "INSERT"),
		),
	)
	defer span.End()

	query, args, err := p.builder.
		Insert("avatars").
		Columns("user_id", "file_name", "mime_type", "s3_key", "size_bytes").
		Values(input.UserID, input.Filename, input.MimeType, input.S3Key, input.Size).
		Suffix("RETURNING id, created_at").
		ToSql()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return CreateAvatarOutput{}, fmt.Errorf("build query: %w", err)
	}

	var output CreateAvatarOutput
	if err := p.pool.QueryRow(ctx, query, args...).Scan(&output.ID, &output.CreatedAt); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return CreateAvatarOutput{}, fmt.Errorf("scan row: %w", err)
	}

	return output, nil
}

func (p *Postgres) GetAvatar(ctx context.Context, id uuid.UUID) (GetAvatarOutput, error) {
	ctx, span := p.tracer.Start(ctx, "SELECT avatars",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system.name", "postgresql"),
			attribute.String("db.collection.name", "avatars"),
			attribute.String("db.operation.name", "SELECT"),
		),
	)
	defer span.End()

	query, args, err := p.builder.
		Select("mime_type", "s3_key").
		From("avatars").
		Where("id = ? AND deleted_at IS NULL", id).
		ToSql()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return GetAvatarOutput{}, fmt.Errorf("build query: %w", err)
	}

	var output GetAvatarOutput
	err = p.pool.QueryRow(ctx, query, args...).Scan(&output.MimeType, &output.S3Key)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return GetAvatarOutput{}, ErrNotFound
	case err != nil:
		span.SetStatus(codes.Error, err.Error())
		return GetAvatarOutput{}, fmt.Errorf("scan row: %w", err)
	}

	return output, nil
}

func (p *Postgres) GetAvatarMetadata(ctx context.Context, id uuid.UUID) (GetAvatarMetadataOutput, error) {
	ctx, span := p.tracer.Start(ctx, "SELECT avatars",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system.name", "postgresql"),
			attribute.String("db.collection.name", "avatars"),
			attribute.String("db.operation.name", "SELECT"),
		),
	)
	defer span.End()

	query, args, err := p.builder.
		Select(
			"id", "user_id", "file_name", "mime_type", "size_bytes",
			"s3_key", "thumbnail_s3_keys", "created_at", "updated_at",
		).
		From("avatars").
		Where("id = ? AND deleted_at IS NULL", id).
		ToSql()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return GetAvatarMetadataOutput{}, fmt.Errorf("build query: %w", err)
	}

	var output GetAvatarMetadataOutput
	err = p.pool.QueryRow(ctx, query, args...).Scan(
		&output.ID, &output.UserID, &output.Filename, &output.MimeType, &output.Size,
		&output.S3Key, &output.ThumbnailS3Keys, &output.CreatedAt, &output.UpdatedAt,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return GetAvatarMetadataOutput{}, ErrNotFound
	case err != nil:
		span.SetStatus(codes.Error, err.Error())
		return GetAvatarMetadataOutput{}, fmt.Errorf("scan row: %w", err)
	}

	return output, nil
}

func (p *Postgres) DeleteAvatar(ctx context.Context, id uuid.UUID) error {
	ctx, span := p.tracer.Start(ctx, "UPDATE avatars",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system.name", "postgresql"),
			attribute.String("db.collection.name", "avatars"),
			attribute.String("db.operation.name", "UPDATE"),
		),
	)
	defer span.End()

	query, args, err := p.builder.
		Update("avatars").
		Set("deleted_at", squirrel.Expr("NOW()")).
		Where("id = ? AND deleted_at IS NULL", id).
		ToSql()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("build query: %w", err)
	}

	if _, err := p.pool.Exec(ctx, query, args...); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("exec query: %w", err)
	}

	return nil
}

func (p *Postgres) AddThumbnailKey(ctx context.Context, input AddThumbnailKeyInput) error {
	ctx, span := p.tracer.Start(ctx, "UPDATE avatars",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system.name", "postgresql"),
			attribute.String("db.collection.name", "avatars"),
			attribute.String("db.operation.name", "UPDATE"),
		),
	)
	defer span.End()

	query, args, err := p.builder.
		Update("avatars").
		Set("thumbnail_s3_keys", squirrel.Expr("thumbnail_s3_keys || to_jsonb(?::text)", input.S3Key)).
		Where("id = ? AND deleted_at IS NULL", input.AvatarID).
		ToSql()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("build query: %w", err)
	}

	if _, err := p.pool.Exec(ctx, query, args...); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("exec query: %w", err)
	}

	return nil
}
