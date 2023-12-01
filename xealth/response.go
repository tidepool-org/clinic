package xealth

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/TwiN/deepmerge"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/xealth_client"
)

type ResponseBuilder[T FormData, E FormErrors] interface {
	WithDataTrackingId(id string) ResponseBuilder[T, E]
	WithDataValidator(validator DataValidator[T, E]) ResponseBuilder[T, E]
	WithData(T) ResponseBuilder[T, E]
	WithRenderedTitleTemplate(template string, vars ...any) ResponseBuilder[T, E]
	WithTitle(string) ResponseBuilder[T, E]
	WithUserInput(userInput *map[string]interface{}) ResponseBuilder[T, E]
	PersistPreorderDataOnSuccess(ctx context.Context, store Store) ResponseBuilder[T, E]
	WithMatchingPatients(criteria PatientMatchingCriteria, patients []*patients.Patient) ResponseBuilder[T, E]
	BuildInitialResponse() (*xealth_client.PreorderFormResponse, error)
	BuildSubsequentResponse() (*xealth_client.PreorderFormResponse, error)
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

type responseBuilder[T FormData, E FormErrors] struct {
	data             T
	dataTrackingId   string
	userInput        *map[string]interface{}
	validator        DataValidator[T, E]
	dataStore        Store
	dataStoreContext context.Context

	criteria         *PatientMatchingCriteria
	matchingPatients []*patients.Patient

	jsonForm      []byte
	formOverrides FormOverrides

	formDataHasErrors     bool
	jsonOverrides         []byte
	jsonFormWithOverrides []byte
	response              xealth_client.PreorderFormResponse
}

func (g *responseBuilder[T, E]) WithDataTrackingId(id string) ResponseBuilder[T, E] {
	g.dataTrackingId = id
	return g
}

func (g *responseBuilder[T, E]) WithDataValidator(validator DataValidator[T, E]) ResponseBuilder[T, E] {
	g.validator = validator
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

func (g *responseBuilder[T, E]) PersistPreorderDataOnSuccess(ctx context.Context, store Store) ResponseBuilder[T, E] {
	g.dataStore = store
	g.dataStoreContext = ctx
	return g
}

func (g *responseBuilder[T, E]) WithMatchingPatients(criteria PatientMatchingCriteria, patients []*patients.Patient) ResponseBuilder[T, E] {
	g.criteria = &criteria
	g.matchingPatients = patients
	return g
}

func (g *responseBuilder[T, E]) BuildInitialResponse() (*xealth_client.PreorderFormResponse, error) {
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

func (g *responseBuilder[T, E]) BuildSubsequentResponse() (*xealth_client.PreorderFormResponse, error) {
	type buildStageFn func() error
	pipeline := []buildStageFn{
		g.assertNoMatchingPatients,
		g.assertFormTemplateIsSet,
		g.maybeDecodeUserInput,
		g.maybeValidateData,
		g.processOverrides,
		g.maybePersistData,
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

func (g *responseBuilder[T, E]) assertNoMatchingPatients() error {
	count := len(g.matchingPatients)
	if count != 0 {
		return fmt.Errorf("expected no patients to match the criteria, but found %v", count)
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
	if g.validator != nil {
		if errors, e := g.validator.Validate(g.data); e != nil {
			err = e
			return
		} else {
			g.formDataHasErrors = errors.HasErrors()
			g.formOverrides.UiSchema = errors
		}
	}
	return
}

func (g *responseBuilder[T, E]) maybePersistData() (err error) {
	if g.dataTrackingId == "" {
		err = fmt.Errorf("data tracking is required")
		return
	}
	if g.dataStore != nil && !g.isErrorResponse() {
		normalized := g.data.Normalize()
		normalized.DataTrackingId = g.dataTrackingId

		err = g.dataStore.CreatePreorderData(g.dataStoreContext, normalized)
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

func (g *responseBuilder[T, E]) buildInitialPreorderFormResponse() (*xealth_client.PreorderFormResponse, error) {
	response := &xealth_client.PreorderFormResponse{}
	initialResponse := xealth_client.PreorderFormResponse0{
		DataTrackingId: g.dataTrackingId,
	}
	if err := g.populatePreorderFormInfo(&initialResponse); err != nil {
		return nil, err
	}

	err := response.FromPreorderFormResponse0(initialResponse)
	return response, err
}

func (g *responseBuilder[T, E]) populatePreorderFormInfo(response *xealth_client.PreorderFormResponse0) (err error) {
	if err = json.Unmarshal(g.jsonFormWithOverrides, &response.PreorderFormInfo); err != nil {
		return
	}

	response.PreorderFormInfo.FormData, err = EncodeFormData(g.data)
	return
}

func (g *responseBuilder[T, E]) isErrorResponse() bool {
	return g.formDataHasErrors
}

func NewFinalResponse() (*xealth_client.PreorderFormResponse, error) {
	response := &xealth_client.PreorderFormResponse{}
	err := response.FromPreorderFormResponse1(xealth_client.PreorderFormResponse1{})
	return response, err
}
