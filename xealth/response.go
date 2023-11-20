package xealth

import (
	"encoding/json"
	"fmt"
	"github.com/TwiN/deepmerge"
	"github.com/tidepool-org/clinic/xealth_models"
)

type ResponseBuilder[T FormData[E], E any] interface {
	WithDataTrackingId(id string) ResponseBuilder[T, E]
	WithDataValidation() ResponseBuilder[T, E]
	WithData(T) ResponseBuilder[T, E]
	WithRenderedTitleTemplate(template string, vars ...any) ResponseBuilder[T, E]
	WithTitle(string) ResponseBuilder[T, E]
	WithUserInput(userInput *map[string]interface{}) ResponseBuilder[T, E]
	BuildInitialResponse() (*xealth_models.PreorderFormResponse, error)
	BuildSubsequentResponse() (*xealth_models.PreorderFormResponse, error)
}

func NewGuardianFlowResponseBuilder() ResponseBuilder[GuardianFormData, GuardianFormValidationErrors] {
	builder := &responseBuilder[GuardianFormData, GuardianFormValidationErrors]{
		jsonForm: guardianEnrollmentForm,
	}
	return builder.WithTitle(DefaultFormTitle)
}

func NewPatientFlowResponseBuilder() ResponseBuilder[PatientFormData, PatientFormValidationErrors] {
	builder := &responseBuilder[PatientFormData, PatientFormValidationErrors]{
		jsonForm: patientEnrollmentForm,
	}
	return builder.WithTitle(DefaultFormTitle)
}

type responseBuilder[T FormData[E], E any] struct {
	data               T
	userInput          *map[string]interface{}
	shouldValidateData bool
	dataTrackingId     string
	jsonForm           []byte
	formOverrides      FormOverrides

	formDataHasErrors     bool
	jsonOverrides         []byte
	jsonFormWithOverrides []byte
	response              xealth_models.PreorderFormResponse
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

func (g *responseBuilder[T, E]) WithTitle(title string) ResponseBuilder[T, E] {
	g.formOverrides.FormSchema.Title = title
	return g
}

func (g *responseBuilder[T, E]) WithRenderedTitleTemplate(template string, vars ...any) ResponseBuilder[T, E] {
	return g.WithTitle(fmt.Sprintf(template, vars...))
}

func (g *responseBuilder[T, E]) WithUserInput(userInput *map[string]interface{}) ResponseBuilder[T, E] {
	g.userInput = userInput
	return g
}

func (g *responseBuilder[T, E]) BuildInitialResponse() (*xealth_models.PreorderFormResponse, error) {
	type buildStageFn func() error
	pipeline := []buildStageFn{
		g.assertFormTemplateIsSet,
		g.processOverrides,
	}

	for _, fn := range pipeline {
		if err := fn(); err != nil {
			return nil, err
		}
	}

	return g.buildInitialPreorderFormResponse()
}

func (g *responseBuilder[T, E]) BuildSubsequentResponse() (*xealth_models.PreorderFormResponse, error) {
	type buildStageFn func() error
	pipeline := []buildStageFn{
		g.assertFormTemplateIsSet,
		g.maybeDecodeUserInput,
		g.maybeValidateData,
		g.processOverrides,
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

func (g *responseBuilder[T, E]) assertFormTemplateIsSet() error {
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
			g.formDataHasErrors = hasError
			g.formOverrides.UiSchema = errors
		}
	}
	return
}

func (g *responseBuilder[T, E]) processOverrides() (err error) {
	// deepmerge.JSON will error out if both the json form (dst) and the overrides (src) define
	// the same key with a primitive type. Make sure the json form template doesn't have keys
	// which are defined in FormOverrides.
	jsonOverrides, err := json.Marshal(g.formOverrides)
	g.jsonFormWithOverrides, err = deepmerge.JSON(g.jsonForm, jsonOverrides)
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
	if err = json.Unmarshal(g.jsonFormWithOverrides, &response.PreorderFormInfo); err != nil {
		return
	}

	response.PreorderFormInfo.FormData, err = EncodeFormData(g.data)
	return
}

func (g *responseBuilder[T, E]) isErrorResponse() bool {
	return g.formDataHasErrors
}

func NewFinalResponse() (*xealth_models.PreorderFormResponse, error) {
	response := &xealth_models.PreorderFormResponse{}
	err := response.FromPreorderFormResponse1(xealth_models.PreorderFormResponse1{})
	return response, err
}
