package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
)

const _healthcheckTimeout = 3 * time.Second

type HealthHandler struct {
	service HealthService
}

type HealthService interface {
	Ping(ctx context.Context) error
}

func NewHealthHandler(service HealthService) *HealthHandler {
	return &HealthHandler{
		service: service,
	}
}

type HealthError struct {
	Error string `json:"error"`
}

func (h *HealthHandler) Health(c *echo.Context) error {
	tctx, tcancel := context.WithTimeout(c.Request().Context(), _healthcheckTimeout)
	defer tcancel()

	if err := h.service.Ping(tctx); err != nil {
		return c.JSON(http.StatusServiceUnavailable, HealthError{
			Error: "Not healthy",
		})
	}

	return c.JSON(http.StatusOK, "Healthy")
}
