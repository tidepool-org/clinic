package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/redox"
	"io"
	"net/http"
)

func (h *Handler) VerifyEndpoint(ec echo.Context) error {
	request := redox.VerificationRequest{}
	if err := ec.Bind(&request); err != nil {
		return err
	}
	result, err := h.redox.VerifyEndpoint(request)
	if err != nil {
		return err
	}

	return ec.JSON(http.StatusOK, result)
}

func (h *Handler) ProcessEHRMessage(ec echo.Context) error {
	ctx := ec.Request().Context()

	// Make sure the request is initiated by redox
	if err := h.redox.AuthorizeRequest(ec.Request()); err != nil {
		return err
	}

	// Capture raw json for later processing
	raw, err := io.ReadAll(ec.Request().Body)
	if err != nil {
		return err
	}

	return h.redox.ProcessEHRMessage(ctx, raw)
}
