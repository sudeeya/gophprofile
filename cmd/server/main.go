package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sudeeya/gophprofile/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	a, err := app.New()
	if err != nil {
		return fmt.Errorf("new app: %w", err)
	}

	if err := a.Run(ctx); err != nil {
		return fmt.Errorf("run app: %w", err)
	}

	return nil
}
