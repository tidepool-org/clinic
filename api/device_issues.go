package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// UpdateDeviceIssues lets backend services trigger a check of patients' device issues.
func (h *Handler) UpdateDeviceIssues(ec echo.Context) error {
	return ec.NoContent(http.StatusNoContent)
}
