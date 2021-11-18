package api

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

type HealthCheck struct {
	ready bool
}

func NewHealthCheck() *HealthCheck {
	return &HealthCheck{}
}

func (h *HealthCheck) SetReady(ready bool) {
	h.ready = ready
}

// Readiness probe
func (h *HealthCheck) Ready(c echo.Context) error {
	if !h.ready {
		return c.NoContent(http.StatusBadRequest)
	}

	return c.NoContent(http.StatusOK)
}



