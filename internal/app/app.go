package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/sudeeya/gophprofile/internal/config"
	"github.com/sudeeya/gophprofile/internal/handlers"
	"github.com/sudeeya/gophprofile/internal/publisher"
	"github.com/sudeeya/gophprofile/internal/repository"
	"github.com/sudeeya/gophprofile/internal/services"
	"github.com/sudeeya/gophprofile/internal/storage"
)

type App struct {
	cfg     config.Config
	server  *http.Server
	closers []io.Closer
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

	publisher, err := publisher.NewRabbitmq()
	if err != nil {
		return nil, fmt.Errorf("new rabbitmq: %w", err)
	}

	healthService := services.NewHealthService(repo, storage, publisher)
	avatarService := services.NewAvatarService(repo, storage, publisher)

	healthHandler := handlers.NewHealthHandler(healthService)
	avatarHandler := handlers.NewAvatarHandler(avatarService)

	handler := echo.New()
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
		cfg:     cfg,
		server:  server,
		closers: closers,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error)

	go func() {
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("listen and serve: %w", err)
	case <-ctx.Done():
	}

	tctx, tcancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
	defer tcancel()

	var closeErrs []error

	if err := a.server.Shutdown(tctx); err != nil {
		closeErrs = append(closeErrs, fmt.Errorf("shutdown server: %w", err))
	}

	for _, closer := range a.closers {
		closeErrs = append(closeErrs, closer.Close())
	}

	return errors.Join(closeErrs...)
}
