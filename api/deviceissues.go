package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) UpdateDeviceIssues(ec echo.Context) error {
	ctx := ec.Request().Context()

	err := h.Patients.UpdateDeviceIssues(ctx)
	if err != nil {
		return ec.JSON(http.StatusInternalServerError, nil)
	}

	return ec.JSON(http.StatusOK, nil)
}
