package xealth

import (
	_ "embed"
	"encoding/json"
	"github.com/tidepool-org/clinic/xealth_models"
)

//go:embed forms/enrollment_step1.json
var enrollmentStep1 []byte

func SetEnrollmentStep1(response *xealth_models.PreorderFormResponse0) error {
	return json.Unmarshal(enrollmentStep1, &response.PreorderFormInfo)
}
