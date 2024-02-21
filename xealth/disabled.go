package xealth

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/xealth_client"
	"net/http"
)

type disabledHandler struct{}

var _ Xealth = &disabledHandler{}

func (d *disabledHandler) GetProgramUrl(ctx context.Context, request xealth_client.GetProgramUrlRequest) (*xealth_client.GetProgramUrlResponse, error) {
	return nil, fmt.Errorf("the xealth integration is not enabled")
}

func (d *disabledHandler) HandleEventNotification(ctx context.Context, event xealth_client.EventNotification) error {
	return fmt.Errorf("the xealth integration is not enabled")
}

func (d *disabledHandler) AuthorizeRequest(req *http.Request) error {
	return fmt.Errorf("the xealth integration is not enabled")
}

func (d *disabledHandler) ProcessInitialPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest0) (*xealth_client.PreorderFormResponse, error) {
	return nil, fmt.Errorf("the xealth integration is not enabled")
}

func (d *disabledHandler) ProcessSubsequentPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest1) (*xealth_client.PreorderFormResponse, error) {
	return nil, fmt.Errorf("the xealth integration is not enabled")
}

func (d *disabledHandler) GetPrograms(ctx context.Context, event xealth_client.GetProgramsRequest) (*xealth_client.GetProgramsResponse, error) {
	return nil, fmt.Errorf("the xealth integration is not enabled")
}

func (d *disabledHandler) GetPDFReport(ctx context.Context, request PDFReportRequest) (*PDFReport, error) {
	return nil, fmt.Errorf("the xealth integration is not enabled")
}
