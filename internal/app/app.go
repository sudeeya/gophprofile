package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/sudeeya/gophprofile/internal/config"
)

type App struct {
	cfg    config.Config
	server *http.Server
}

func New() (*App, error) {
	cfg, err := config.New()
	if err != nil {
		return nil, fmt.Errorf("new config: %w", err)
	}

	server := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: echo.New(),
	}

	return &App{
		cfg:    cfg,
		server: server,
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

	if err := a.server.Shutdown(tctx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	return nil
}
