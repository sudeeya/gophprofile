package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"golang.org/x/sync/errgroup"

	"github.com/sudeeya/gophprofile/internal/broker"
	"github.com/sudeeya/gophprofile/internal/repository"
	"github.com/sudeeya/gophprofile/internal/services"
	"github.com/sudeeya/gophprofile/internal/storage"
	"github.com/sudeeya/gophprofile/internal/worker/config"
)

type App struct {
	cfg      config.Config
	consumer *broker.RabbitmqConsumer
	closers  []io.Closer
}

func New(ctx context.Context) (*App, error) {
	var closers []io.Closer

	cfg, err := config.New()
	if err != nil {
		return nil, fmt.Errorf("new config: %w", err)
	}

	repo, err := repository.NewPostgres(ctx, repository.PostgresConfig{
		Host:     cfg.Postgres.Host,
		Port:     cfg.Postgres.Port,
		User:     cfg.Postgres.User,
		Password: cfg.Postgres.Password,
		DB:       cfg.Postgres.DB,
	})
	if err != nil {
		return nil, fmt.Errorf("new postgres: %w", err)
	}
	closers = append(closers, repo)

	storage, err := storage.NewMinio(ctx, storage.MinioConfig{
		Endpoint: cfg.Minio.Endpoint,
		User:     cfg.Minio.User,
		Password: cfg.Minio.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("new minio: %w", err)
	}

	thumbnailService := services.NewAvatarThumbnailService(repo, storage)

	consumer, err := broker.NewRabbitmqConsumer(broker.RabbitmqConsumerConfig{
		Host:     cfg.Rabbitmq.Host,
		Port:     cfg.Rabbitmq.Port,
		User:     cfg.Rabbitmq.User,
		Password: cfg.Rabbitmq.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("new rabbitmq consumer: %w", err)
	}
	closers = append(closers, consumer)

	consumer.Register(broker.QueueAvatarUpload, func(ctx context.Context, event broker.AvatarUploadEvent) error {
		return thumbnailService.GenerateThumbnail(ctx, event.ID)
	})

	consumer.Register(broker.QueueAvatarDelete, func(ctx context.Context, event broker.AvatarDeleteEvent) error {
		return thumbnailService.DeleteAvatar(ctx, event.S3Keys)
	})

	return &App{
		cfg:      cfg,
		consumer: consumer,
		closers:  closers,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := a.consumer.Run(gctx); err != nil {
			return fmt.Errorf("run consumer: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	var closeErrs []error

	for _, closer := range a.closers {
		closeErrs = append(closeErrs, closer.Close())
	}

	return errors.Join(closeErrs...)
}
