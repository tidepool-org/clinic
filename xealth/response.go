package xealth

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/TwiN/deepmerge"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/xealth_client"
	"sort"
	"strings"
)

type ResponseBuilder[T FormData] interface {
	WithDataTrackingId(id string) ResponseBuilder[T]
	WithDataValidator(validator DataValidator[T]) ResponseBuilder[T]
	WithData(T) ResponseBuilder[T]
	WithRenderedTitleTemplate(template string, vars ...any) ResponseBuilder[T]
	WithTags([]clinics.PatientTag) ResponseBuilder[T]
	WithTitle(string) ResponseBuilder[T]
	WithUserInput(userInput *map[string]interface{}) ResponseBuilder[T]
	PersistPreorderDataOnSuccess(ctx context.Context, store Store) ResponseBuilder[T]
	WithMatchingPatients(criteria PatientMatchingCriteria, patients []*patients.Patient) ResponseBuilder[T]
	BuildInitialResponse() (*xealth_client.PreorderFormResponse, error)
	BuildSubsequentResponse() (*xealth_client.PreorderFormResponse, error)
}

func NewGuardianFlowResponseBuilder() ResponseBuilder[GuardianFormData] {
	builder := &responseBuilder[GuardianFormData]{
		jsonForm: guardianEnrollmentForm,
	}
	return builder
}

func NewPatientFlowResponseBuilder() ResponseBuilder[PatientFormData] {
	builder := &responseBuilder[PatientFormData]{
		jsonForm: patientEnrollmentForm,
	}
	return builder
}

type responseBuilder[T FormData] struct {
	data             T
	dataTrackingId   string
	userInput        *map[string]interface{}
	validator        DataValidator[T]
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

func (g *responseBuilder[T]) WithDataTrackingId(id string) ResponseBuilder[T] {
	g.dataTrackingId = id
	return g
}

func (g *responseBuilder[T]) WithDataValidator(validator DataValidator[T]) ResponseBuilder[T] {
	g.validator = validator
	return g
}

func (g *responseBuilder[T]) WithData(data T) ResponseBuilder[T] {
	g.data = data
	return g
}

func (g *responseBuilder[T]) WithTitle(title string) ResponseBuilder[T] {
	g.formOverrides.FormSchema.Title = title
	return g
}

func (g *responseBuilder[T]) WithRenderedTitleTemplate(template string, vars ...any) ResponseBuilder[T] {
	return g.WithTitle(fmt.Sprintf(template, vars...))
}

func (g *responseBuilder[T]) WithUserInput(userInput *map[string]interface{}) ResponseBuilder[T] {
	g.userInput = userInput
	return g
}

func (g *responseBuilder[T]) WithTags(tags []clinics.PatientTag) ResponseBuilder[T] {
	if len(tags) > 0 {
		sort.Slice(tags, func(i, j int) bool { return strings.Compare(tags[i].Name, tags[j].Name) < 0 })

		enum := make([]string, 0, len(tags))
		enumNames := make([]string, 0, len(tags))
		for _, t := range tags {
			if t.Id != nil {
				enum = append(enum, t.Id.Hex())
				enumNames = append(enumNames, t.Name)
			}
		}
		g.formOverrides.FormSchema.Definitions.Tags.Enum = enum
		g.formOverrides.FormSchema.Definitions.Tags.EnumNames = enumNames
	}

	return g
}

func (g *responseBuilder[T]) PersistPreorderDataOnSuccess(ctx context.Context, store Store) ResponseBuilder[T] {
	g.dataStore = store
	g.dataStoreContext = ctx
	return g
}

func (g *responseBuilder[T]) WithMatchingPatients(criteria PatientMatchingCriteria, patients []*patients.Patient) ResponseBuilder[T] {
	g.criteria = &criteria
	g.matchingPatients = patients
	return g
}

func (g *responseBuilder[T]) BuildInitialResponse() (*xealth_client.PreorderFormResponse, error) {
	type buildStageFn func() error
	pipeline := []buildStageFn{
		g.assertFormTemplateIsSet,
		g.maybeShowTags,
		g.processOverrides,
	}

	for _, fn := range pipeline {
		if err := fn(); err != nil {
			return nil, err
		}
	}

	return g.buildInitialPreorderFormResponse()
}

func (g *responseBuilder[T]) BuildSubsequentResponse() (*xealth_client.PreorderFormResponse, error) {
	type buildStageFn func() error
	pipeline := []buildStageFn{
		g.assertNoMatchingPatients,
		g.assertFormTemplateIsSet,
		g.maybeDecodeUserInput,
		g.maybeValidateData,
		g.maybeShowTags,
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

func (g *responseBuilder[T]) assertFormTemplateIsSet() error {
	if len(g.jsonForm) == 0 {
		return fmt.Errorf("the form template is required")
	}
	return nil
}

func (g *responseBuilder[T]) assertNoMatchingPatients() error {
	count := len(g.matchingPatients)
	if count != 0 {
		return fmt.Errorf("expected no patients to match the criteria, but found %v", count)
	}
	return nil
}

func (g *responseBuilder[T]) maybeDecodeUserInput() (err error) {
	if g.userInput != nil {
		g.data, err = DecodeFormData[T](g.userInput)
	}
	return
}

func (g *responseBuilder[T]) maybeValidateData() (err error) {
	if g.validator != nil {
		if errors, e := g.validator.Validate(g.data); e != nil {
			err = e
			return
		} else {
			g.formDataHasErrors = errors.HasErrors()
			if g.formDataHasErrors {
				g.jsonForm = errorForm
				if title := errors.GetTitle(); title != "" {
					g.formOverrides.FormSchema.Title = errors.GetTitle()
				}
				errorProperties := errors.GetErrorProperties()
				if len(errorProperties) > 0 {
					if len(g.formOverrides.FormSchema.Properties) == 0 {
						g.formOverrides.FormSchema.Properties = make(map[string]interface{})
					}
					for k, v := range errorProperties {
						g.formOverrides.FormSchema.Properties[k] = v
					}
				}
				g.formOverrides.UiSchema.UiOrder = errors.GetUiOrder()
			}
		}
	}
	return
}

func (g *responseBuilder[T]) maybePersistData() (err error) {
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

func (g *responseBuilder[T]) maybeShowTags() (err error) {
	if !g.isErrorResponse() {
		if len(g.formOverrides.FormSchema.Definitions.Tags.Enum) > 0 && len(g.formOverrides.FormSchema.Definitions.Tags.EnumNames) > 0 {
			if g.formOverrides.FormSchema.Properties == nil {
				g.formOverrides.FormSchema.Properties = make(map[string]interface{})
			}
			// Set uiSchema/tags["ui:widget"]: "hidden" to "object"
			g.formOverrides.UiSchema.Tags.UiWidget = "object"
			// Set formSchema/properties["tags"]/type=object
			g.formOverrides.FormSchema.Properties["tags"] = struct {
				Type string `json:"type"`
			}{
				Type: "object",
			}
		}
	}
	return
}

func (g *responseBuilder[T]) processOverrides() (err error) {
	jsonOverrides, err := json.Marshal(g.formOverrides)
	g.jsonFormWithOverrides, err = deepmerge.JSON(g.jsonForm, jsonOverrides, deepmerge.Config{PreventMultipleDefinitionsOfKeysWithPrimitiveValue: false})
	return
}

func (g *responseBuilder[T]) buildInitialPreorderFormResponse() (*xealth_client.PreorderFormResponse, error) {
	response := &xealth_client.PreorderFormResponse{}
	notOrderable := g.isErrorResponse()
	initialResponse := xealth_client.PreorderFormResponse0{
		DataTrackingId: g.dataTrackingId,
		NotOrderable:   &notOrderable,
	}

	if err := g.populatePreorderFormInfo(&initialResponse); err != nil {
		return nil, err
	}

	err := response.FromPreorderFormResponse0(initialResponse)
	return response, err
}

func (g *responseBuilder[T]) populatePreorderFormInfo(response *xealth_client.PreorderFormResponse0) (err error) {
	if err = json.Unmarshal(g.jsonFormWithOverrides, &response.PreorderFormInfo); err != nil {
		return
	}

	response.PreorderFormInfo.FormData, err = EncodeFormData(g.data)
	return
}

func (g *responseBuilder[T]) isErrorResponse() bool {
	return g.formDataHasErrors
}

func NewFinalResponse() (*xealth_client.PreorderFormResponse, error) {
	response := &xealth_client.PreorderFormResponse{}
	err := response.FromPreorderFormResponse1(xealth_client.PreorderFormResponse1{})
	return response, err
}
