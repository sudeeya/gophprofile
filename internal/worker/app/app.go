package app

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/sudeeya/gophprofile/internal/broker"
	"github.com/sudeeya/gophprofile/internal/otel"
	"github.com/sudeeya/gophprofile/internal/repository"
	"github.com/sudeeya/gophprofile/internal/services"
	"github.com/sudeeya/gophprofile/internal/storage"
	"github.com/sudeeya/gophprofile/internal/worker/config"
)

type App struct {
	cfg             config.Config
	consumer        *broker.RabbitmqConsumer
	onShutdownFuncs []func(context.Context) error
}

func New(ctx context.Context) (*App, error) {
	var onShutdownFuncs []func(context.Context) error

	cfg, err := config.New()
	if err != nil {
		return nil, fmt.Errorf("new config: %w", err)
	}

	stopTracerProvider, err := otel.InitTracerProvider(ctx)
	if err != nil {
		return nil, fmt.Errorf("init tracer provider: %w", err)
	}
	onShutdownFuncs = append(onShutdownFuncs, func(ctx context.Context) error {
		if err := stopTracerProvider(ctx); err != nil {
			return fmt.Errorf("stop tracer provider: %w", err)
		}
		return nil
	})

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
	onShutdownFuncs = append(onShutdownFuncs, func(_ context.Context) error {
		if err := repo.Close(); err != nil {
			return fmt.Errorf("close postgres: %w", err)
		}
		return nil
	})

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
	onShutdownFuncs = append(onShutdownFuncs, func(_ context.Context) error {
		if err := consumer.Close(); err != nil {
			return fmt.Errorf("close consumer: %w", err)
		}
		return nil
	})

	consumer.Register(broker.QueueAvatarUpload, func(ctx context.Context, event broker.AvatarUploadEvent) error {
		return thumbnailService.GenerateThumbnail(ctx, event.ID)
	})

	consumer.Register(broker.QueueAvatarDelete, func(ctx context.Context, event broker.AvatarDeleteEvent) error {
		return thumbnailService.DeleteAvatar(ctx, event.S3Keys)
	})

	return &App{
		cfg:             cfg,
		consumer:        consumer,
		onShutdownFuncs: onShutdownFuncs,
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

	tctx, tcancel := context.WithTimeout(context.Background(), a.cfg.Worker.ShutdownTimeout)
	defer tcancel()

	var shutdownErrs []error

	for _, f := range a.onShutdownFuncs {
		shutdownErrs = append(shutdownErrs, f(tctx))
	}

	return errors.Join(shutdownErrs...)
}
