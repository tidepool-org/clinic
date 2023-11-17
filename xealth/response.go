package xealth

import (
	"encoding/json"
	"fmt"
	"github.com/RaveNoX/go-jsonmerge"
	"github.com/tidepool-org/clinic/xealth_models"
)

type ResponseBuilder[T Validatable[E], E any] interface {
	WithFormTemplate(form []byte) ResponseBuilder[T, E]
	WithDataTrackingId(id string) ResponseBuilder[T, E]
	WithDataValidation() ResponseBuilder[T, E]
	WithData(T) ResponseBuilder[T, E]
	WithUserInput(userInput *map[string]interface{}) ResponseBuilder[T, E]
	BuildInitialResponse() (*xealth_models.PreorderFormResponse, error)
	BuildSubsequentResponse() (*xealth_models.PreorderFormResponse, error)
}

func NewGuardianFlowResponseBuilder() ResponseBuilder[GuardianFormData, GuardianFormValidationErrors] {
	return &responseBuilder[GuardianFormData, GuardianFormValidationErrors]{
		jsonForm: guardianEnrollmentForm,
	}
}

func NewPatientFlowResponseBuilder() ResponseBuilder[PatientFormData, PatientFormValidationErrors] {
	return &responseBuilder[PatientFormData, PatientFormValidationErrors]{
		jsonForm: patientEnrollmentForm,
	}
}

type responseBuilder[T Validatable[E], E any] struct {
	data               T
	userInput          *map[string]interface{}
	shouldValidateData bool
	dataTrackingId     string
	jsonForm           []byte

	jsonErrors         []byte
	jsonFormWithErrors []byte
	response           xealth_models.PreorderFormResponse
}

func (g *responseBuilder[T, E]) WithFormTemplate(form []byte) ResponseBuilder[T, E] {
	g.jsonForm = form
	return g
}

func (g *responseBuilder[T, E]) WithDataTrackingId(id string) ResponseBuilder[T, E] {
	g.dataTrackingId = id
	return g
}

func (g *responseBuilder[T, E]) WithDataValidation() ResponseBuilder[T, E] {
	g.shouldValidateData = true
	return g
}

func (g *responseBuilder[T, E]) WithData(data T) ResponseBuilder[T, E] {
	g.data = data
	return g
}

func (g *responseBuilder[T, E]) WithUserInput(userInput *map[string]interface{}) ResponseBuilder[T, E] {
	g.userInput = userInput
	return g
}

func (g *responseBuilder[T, E]) BuildInitialResponse() (*xealth_models.PreorderFormResponse, error) {
	if err := g.assertFromTemplateIsSet(); err != nil {
		return nil, err
	}

	return g.buildInitialPreorderFormResponse()
}

func (g *responseBuilder[T, E]) BuildSubsequentResponse() (*xealth_models.PreorderFormResponse, error) {
	type buildStageFn func() error
	pipeline := []buildStageFn{
		g.assertFromTemplateIsSet,
		g.maybeDecodeUserInput,
		g.maybeValidateData,
		g.maybeMergeErrors,
	}

	for _, fn := range pipeline {
		if err := fn(); err != nil {
			return nil, err
		}
	}

	// Validation failed, we should return the form with the populated data
	// and the validation errors
	if g.isErrorResponse() {
		return g.buildInitialPreorderFormResponse()
	}

	// There were no errors, send an empty response (response1) to notify Xealth
	// that all data required for fulfilling the order has been gathered
	return NewFinalResponse()
}

func (g *responseBuilder[T, E]) assertFromTemplateIsSet() error {
	if len(g.jsonForm) == 0 {
		return fmt.Errorf("the form template is required")
	}
	return nil
}

func (g *responseBuilder[T, E]) maybeDecodeUserInput() (err error) {
	if g.userInput != nil {
		g.data, err = DecodeFormData[T](g.userInput)
	}
	return
}

func (g *responseBuilder[T, E]) maybeValidateData() (err error) {
	if g.shouldValidateData {
		if hasError, errors := g.data.Validate(); hasError {
			g.jsonErrors, err = json.Marshal(ValidationErrors{
				UiSchema: errors,
			})
		}
	}
	return
}

func (g *responseBuilder[T, E]) maybeMergeErrors() (err error) {
	if g.isErrorResponse() {
		g.jsonFormWithErrors, _, err = jsonmerge.MergeBytes(g.jsonForm, g.jsonErrors)
	}
	return
}

func (g *responseBuilder[T, E]) buildInitialPreorderFormResponse() (*xealth_models.PreorderFormResponse, error) {
	response := &xealth_models.PreorderFormResponse{}
	initialResponse := xealth_models.PreorderFormResponse0{
		DataTrackingId: g.dataTrackingId,
	}
	if err := g.populatePreorderFormInfo(&initialResponse); err != nil {
		return nil, err
	}

	err := response.FromPreorderFormResponse0(initialResponse)
	return response, err
}

func (g *responseBuilder[T, E]) populatePreorderFormInfo(response *xealth_models.PreorderFormResponse0) (err error) {
	jsonForm := g.jsonForm
	if g.isErrorResponse() {
		jsonForm = g.jsonFormWithErrors
	}

	if err = json.Unmarshal(jsonForm, &response.PreorderFormInfo); err != nil {
		return
	}

	response.PreorderFormInfo.FormData, err = EncodeFormData(g.data)
	return
}

func (g *responseBuilder[T, E]) isErrorResponse() bool {
	return len(g.jsonErrors) > 0
}

func NewFinalResponse() (*xealth_models.PreorderFormResponse, error) {
	response := &xealth_models.PreorderFormResponse{}
	err := response.FromPreorderFormResponse1(xealth_models.PreorderFormResponse1{})
	return response, err
}
