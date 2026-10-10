package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	echootel "github.com/labstack/echo-otel/v5"
	"github.com/labstack/echo/v5"
	"golang.org/x/sync/errgroup"

	"github.com/sudeeya/gophprofile/internal/broker"
	"github.com/sudeeya/gophprofile/internal/handlers"
	"github.com/sudeeya/gophprofile/internal/otel"
	"github.com/sudeeya/gophprofile/internal/repository"
	"github.com/sudeeya/gophprofile/internal/server/config"
	"github.com/sudeeya/gophprofile/internal/services"
	"github.com/sudeeya/gophprofile/internal/storage"
)

type App struct {
	cfg             config.Config
	server          *http.Server
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

	publisher, err := broker.NewRabbitmqPublisher(broker.RabbitmqPublisherConfig{
		Host:                           cfg.Rabbitmq.Host,
		Port:                           cfg.Rabbitmq.Port,
		User:                           cfg.Rabbitmq.User,
		Password:                       cfg.Rabbitmq.Password,
		PublisherConfirmsRetryDelay:    cfg.Rabbitmq.PublisherConfirmsRetryDelay,
		PublisherConfirmsRetryAttempts: cfg.Rabbitmq.PublisherConfirmsRetryAttempts,
	})
	if err != nil {
		return nil, fmt.Errorf("new rabbitmq publisher: %w", err)
	}
	onShutdownFuncs = append(onShutdownFuncs, func(_ context.Context) error {
		if err := publisher.Close(); err != nil {
			return fmt.Errorf("close publisher: %w", err)
		}
		return nil
	})

	healthService := services.NewHealthService(repo, storage, publisher)
	avatarService := services.NewAvatarService(repo, storage, publisher)

	healthHandler := handlers.NewHealthHandler(healthService)
	avatarHandler := handlers.NewAvatarHandler(avatarService)

	handler := echo.New()
	handler.Use(echootel.NewMiddleware("server"))
	handler.GET("/health", healthHandler.Health)
	v1 := handler.Group("api/v1")
	v1.POST("/avatars", avatarHandler.UploadAvatar)
	v1.GET("/avatars/:id", avatarHandler.GetAvatar)
	v1.GET("/avatars/:id/metadata", avatarHandler.GetAvatarMetadata)
	v1.DELETE("/avatars/:id", avatarHandler.DeleteAvatar)

	server := &http.Server{
		Addr:    net.JoinHostPort("", cfg.Server.Port),
		Handler: handler,
	}

	return &App{
		cfg:             cfg,
		server:          server,
		onShutdownFuncs: onShutdownFuncs,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := a.server.ListenAndServe(); err != nil {
			return fmt.Errorf("listen and serve: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	tctx, tcancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
	defer tcancel()

	var shutdownErrs []error

	if err := a.server.Shutdown(tctx); err != nil {
		shutdownErrs = append(shutdownErrs, fmt.Errorf("shutdown server: %w", err))
	}

	for _, f := range a.onShutdownFuncs {
		shutdownErrs = append(shutdownErrs, f(tctx))
	}

	return errors.Join(shutdownErrs...)
}
