package services

import (
	"context"
	"errors"
	"sync"
)

type HealthService struct {
	pingers []Pinger
}

type Pinger interface {
	Ping(ctx context.Context) error
}

func NewHealthService(pingers ...Pinger) *HealthService {
	return &HealthService{
		pingers: pingers,
	}
}

func (s *HealthService) Ping(ctx context.Context) error {
	var (
		wg   = &sync.WaitGroup{}
		mu   = &sync.Mutex{}
		errs []error
	)

	for _, pinger := range s.pingers {
		wg.Go(func() {
			if err := pinger.Ping(ctx); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		})
	}

	wg.Wait()

	return errors.Join(errs...)
}
