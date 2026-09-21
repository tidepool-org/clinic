package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) UpdateConnectionIssues(ec echo.Context) error {
	ctx := ec.Request().Context()

	if err := h.Patients.UpdateConnectionIssues(ctx); err != nil {
		return err
	}

	return ec.NoContent(http.StatusNoContent)
}
