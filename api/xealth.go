package api

import (
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/xealth_models"
	"io"
	"net/http"
)

func (h *Handler) XealthPreorder(ec echo.Context) error {
	ctx := ec.Request().Context()

	// Make sure the request is initiated by xealth
	if err := h.xealth.AuthorizeRequest(ec.Request()); err != nil {
		return err
	}

	// Capture raw json for later processing
	raw, err := io.ReadAll(ec.Request().Body)
	if err != nil {
		return err
	}

	type meta struct {
		EventType    string `json:"eventType"`
		EventContext string `json:"eventContext"`
	}
	eventMeta := meta{}
	if err := json.Unmarshal(raw, &eventMeta); err != nil {
		return err
	}

	request := &xealth_models.PreorderFormRequest{}
	if err := json.Unmarshal(raw, request); err != nil {
		return err
	}

	if eventMeta.EventType != string(xealth_models.PreorderFormRequest0EventTypePreorder) {
		return fmt.Errorf("%w: expected eventType='preorder' got %s", errors.BadRequest, eventMeta.EventType)
	}

	switch eventMeta.EventContext {
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
		return fmt.Errorf("%w: invalid event context %s", errors.BadRequest, eventMeta.EventContext)
	}
}