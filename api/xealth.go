package api

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/xealth_models"
	"net/http"
)

func (h *Handler) XealthPreorder(ec echo.Context) error {
	ctx := ec.Request().Context()

	// Make sure the request is initiated by redox
	if err := h.xealth.AuthorizeRequest(ec.Request()); err != nil {
		return err
	}

	type r struct {
		xealth_models.PreorderFormRequest
		EventContext string `json:"eventContext"`
		EventType    string `json:"eventType"`
	}
	request := &r{}
	if err := ec.Bind(&request); err != nil {
		return err
	}

	if request.EventType != string(xealth_models.PreorderFormRequest0EventTypePreorder) {
		return fmt.Errorf("%w: expected eventType='preorder'", errors.BadRequest)
	}

	switch request.EventContext {
	case string(xealth_models.PreorderFormRequest0EventContextInitial):
		initial, err := request.AsPreorderFormRequest0()
		if err != nil {
			return err
		}

		response, err := h.xealth.ProcessInitialPreorderRequest(ctx, initial)
		if err != nil {
			return err
		}
		return ec.JSON(http.StatusOK, response)
	case string(xealth_models.PreorderFormRequest1EventContextSubsequent):
		subsequent, err := request.AsPreorderFormRequest1()
		if err != nil {
			return err
		}

		response, err := h.xealth.ProcessSubsequentPreorderRequest(ctx, subsequent)
		if err != nil {
			return err
		}
		return ec.JSON(http.StatusOK, response)
	default:
		return fmt.Errorf("%w: invalid event context %s", errors.BadRequest, request.EventContext)
	}
}
