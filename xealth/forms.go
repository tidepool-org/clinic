package xealth

import (
	_ "embed"
	"encoding/json"
	"github.com/tidepool-org/clinic/xealth_models"
)

//go:embed forms/enrollment_step1.json
var enrollmentStep1 []byte

//go:embed forms/enrollment_under13_step1.json
var enrollmentUnder13Step1 []byte

func SetEnrollmentStep1(response *xealth_models.PreorderFormResponse0, formData *map[string]interface{}) error {
	if err := json.Unmarshal(enrollmentStep1, &response.PreorderFormInfo); err != nil {
		return err
	}
	response.PreorderFormInfo.FormData = formData
	return nil
}

func SetEnrollmentUnder13Step1(response *xealth_models.PreorderFormResponse0, formData *map[string]interface{}) error {
	if err := json.Unmarshal(enrollmentUnder13Step1, &response.PreorderFormInfo); err != nil {
		return err
	}
	response.PreorderFormInfo.FormData = formData
	return nil
}
