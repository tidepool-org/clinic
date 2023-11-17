package xealth

import (
	_ "embed"
	"encoding/json"
	"github.com/mitchellh/mapstructure"
	"net/mail"
	"strings"
)

//go:embed forms/patient_enrollment_form.json
var patientEnrollmentForm []byte

//go:embed forms/guardian_enrollment_form.json
var guardianEnrollmentForm []byte

//
//func PopulatePatientEnrollmentForm(response *xealth_models.PreorderFormResponse0, data PatientFormData, errors PatientFormValidationErrors) error {
//	return populateEnrollmentForm(response, patientEnrollmentForm, data, errors)
//}
//

//
//func PopulateGuardianEnrollmentForm(response *xealth_models.PreorderFormResponse0, data GuardianFormData, errors GuardianFormValidationErrors) error {
//	return populateEnrollmentForm(response, guardianEnrollmentForm, data, errors)
//}
//
//func populateEnrollmentForm[T any, E any](response *xealth_models.PreorderFormResponse0, formFixture []byte, data T, errors E) error {
//	jsonErrors, err := json.Marshal(errors)
//	if err != nil {
//		return err
//	}
//
//	formWithErrors, _, err := jsonmerge.MergeBytes(formFixture, jsonErrors)
//	if err != nil {
//		return err
//	}
//
//	if err := json.Unmarshal(formWithErrors, &response.PreorderFormInfo); err != nil {
//		return err
//	}
//	formData, err := EncodeFormData(data)
//	if err != nil {
//		return err
//	}
//	response.PreorderFormInfo.FormData = formData
//
//	return nil
//}

type Validatable[E any] interface {
	Validate() (bool, E)
}

type GuardianFormData struct {
	Guardian struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
	} `json:"guardian"`
	Dexcom Dexcom `json:"dexcom"`
}

func (g GuardianFormData) Validate() (bool, GuardianFormValidationErrors) {
	hasError := false
	errors := GuardianFormValidationErrors{}
	if strings.TrimSpace(g.Guardian.FirstName) == "" {
		hasError = true
		errors.Guardian.FirstName = NewValidationError("First name is required")
	}
	if strings.TrimSpace(g.Guardian.LastName) == "" {
		hasError = true
		errors.Guardian.LastName = NewValidationError("Last name is required")
	}
	if !isValidEmail(g.Guardian.Email) {
		hasError = true
		errors.Guardian.Email = NewValidationError("The email address is not valid")
	}
	return hasError, errors
}

type GuardianFormValidationErrors struct {
	Guardian struct {
		FirstName *ValidationError `json:"firstName,omitempty"`
		LastName  *ValidationError `json:"lastName,omitempty"`
		Email     *ValidationError `json:"email,omitempty"`
	} `json:"guardian"`
}

type PatientFormData struct {
	Patient struct {
		Email string `json:"email"`
	} `json:"patient"`
}

func (p PatientFormData) Validate() (bool, PatientFormValidationErrors) {
	hasError := false
	errors := PatientFormValidationErrors{}
	if !isValidEmail(p.Patient.Email) {
		hasError = true
		errors.Patient.Email = NewValidationError("The email address is not valid")
	}

	return hasError, errors
}

type PatientFormValidationErrors struct {
	Patient struct {
		Email *ValidationError `json:"email,omitempty"`
	} `json:"patient"`
}

type Dexcom struct {
	Connect bool `json:"connect"`
}

func DecodeFormData[T any](formData *map[string]interface{}) (data T, err error) {
	if formData == nil || len(*formData) == 0 {
		return
	}

	err = mapstructure.Decode(formData, &data)
	return
}

func EncodeFormData[A any](data A) (*map[string]interface{}, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	formData := make(map[string]interface{})
	err = json.Unmarshal(j, &formData)
	return &formData, err
}

type ValidationErrors struct {
	UiSchema any `json:"uiSchema"`
}

type ValidationError struct {
	UiAutofocus bool `json:"ui:autofocus"`
	UiOptions   struct {
		ErrorMessage string `json:"errorMessage"`
	} `json:"ui:options"`
}

func NewValidationError(message string) *ValidationError {
	err := &ValidationError{}
	err.UiOptions.ErrorMessage = message
	return err
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}
