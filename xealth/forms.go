package xealth

import (
	_ "embed"
	"encoding/json"
	"github.com/mitchellh/mapstructure"
	"net/mail"
)

const (
	DefaultFormTitle             = "Add patient to Tidepool"
	FormTitlePatientNameTemplate = "Add %s to Tidepool"
)

//go:embed forms/patient_enrollment_form.json
var patientEnrollmentForm []byte

//go:embed forms/guardian_enrollment_form.json
var guardianEnrollmentForm []byte

type DataValidator[D any, E FormErrors] interface {
	Validate(D) (E, error)
}

type FormErrors interface {
	HasErrors() bool
}

type GuardianFormData struct {
	Guardian struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
	} `json:"guardian"`
	Dexcom Dexcom `json:"dexcom"`
}

type GuardianFormValidationErrors struct {
	FormHasErrors bool `json:"-"`
	Guardian      struct {
		FirstName *ValidationError `json:"firstName,omitempty"`
		LastName  *ValidationError `json:"lastName,omitempty"`
		Email     *ValidationError `json:"email,omitempty"`
	} `json:"guardian"`
}

func (p GuardianFormValidationErrors) HasErrors() bool {
	return p.FormHasErrors
}

type PatientFormData struct {
	Patient struct {
		Email string `json:"email"`
	} `json:"patient"`
	Dexcom Dexcom `json:"dexcom"`
}

type PatientFormValidationErrors struct {
	FormHasErrors bool `json:"-"`
	Patient       struct {
		Email *ValidationError `json:"email,omitempty"`
	} `json:"patient"`
}

func (p PatientFormValidationErrors) HasErrors() bool {
	return p.FormHasErrors
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

type FormOverrides struct {
	FormSchema FormSchemaOverride `json:"formSchema"`
	UiSchema   any                `json:"uiSchema,omitempty"`
}

type FormSchemaOverride struct {
	Title string `json:"title"`
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
