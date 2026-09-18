package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"
)

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
	if err := h.service.Ping(c.Request().Context()); err != nil {
		return c.JSON(http.StatusServiceUnavailable, HealthError{
			Error: "Not healthy",
		})
	}

	return c.String(http.StatusOK, "Healthy")
}
