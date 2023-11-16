package xealth

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	"github.com/oapi-codegen/runtime/types"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/xealth_models"
	"net/http"
	"strings"
	"time"
)

const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
	emailSystem         = "email"
)

type ModuleConfig struct {
	Enabled bool `envconfig:"TIDEPOOL_XEALTH_ENABLED"`
}

type ClientConfig struct {
	BearerToken  string `envconfig:"TIDEPOOL_XEALTH_BEARER_TOKEN" required:"true"`
	ClientId     string `envconfig:"TIDEPOOL_XEALTH_CLIENT_ID" required:"true"`
	ClientSecret string `envconfig:"TIDEPOOL_XEALTH_CLIENT_SECRET" required:"true"`
}

type Xealth interface {
	AuthorizeRequest(req *http.Request) error
	ProcessInitialPreorderRequest(ctx context.Context, request xealth_models.PreorderFormRequest0) (*xealth_models.PreorderFormResponse, error)
	ProcessSubsequentPreorderRequest(ctx context.Context, request xealth_models.PreorderFormRequest1) (*xealth_models.PreorderFormResponse, error)
}

type defaultHandler struct {
	config *ClientConfig

	clinics  clinics.Service
	patients patients.Service
}

func NewHandler(clinics clinics.Service, patients patients.Service) (Xealth, error) {
	cfg := ModuleConfig{}
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	if !cfg.Enabled {
		return &disabledHandler{}, nil
	}

	clientConfig := &ClientConfig{}
	if err := envconfig.Process("", clientConfig); err != nil {
		return nil, err
	}

	return &defaultHandler{
		config:   clientConfig,
		clinics:  clinics,
		patients: patients,
	}, nil
}

func (h *defaultHandler) ProcessInitialPreorderRequest(ctx context.Context, request xealth_models.PreorderFormRequest0) (*xealth_models.PreorderFormResponse, error) {
	clinic, err := h.FindMatchingClinic(ctx, request.Deployment)
	if err != nil {
		return nil, err
	} else if clinic == nil {
		return nil, fmt.Errorf("%w: couldn't find matching clinic", errors.NotFound)
	}

	criteria, err := GetPatientMatchingCriteria(request.Datasets, clinic)
	if err != nil {
		return nil, err
	} else if criteria == nil {
		return nil, nil
	}

	matchingPatients, err := h.FindMatchingPatients(ctx, criteria, clinic)
	if err != nil {
		return nil, err
	}

	response := &xealth_models.PreorderFormResponse{}
	if count := len(matchingPatients); count == 1 {
		if err := response.FromPreorderFormResponse1(xealth_models.PreorderFormResponse1{}); err != nil {
			return nil, err
		}
	} else if count == 0 {
		formResponse := xealth_models.PreorderFormResponse0{
			DataTrackingId: uuid.NewString(),
		}
		if criteria.IsPatientUnder13() {
			if err := PopulateGuardianEnrollmentForm(&formResponse, GuardianFormData{}, GuardianFormValidationErrors{}); err != nil {
				return nil, err
			}
		} else {
			formData := PatientFormData{}
			formData.Patient.Email = criteria.Email
			if err := PopulatePatientEnrollmentForm(&formResponse, formData, PatientFormValidationErrors{}); err != nil {
				return nil, err
			}
		}

		if err := response.FromPreorderFormResponse0(formResponse); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("%w: multiple matching patients were found", errors.BadRequest)
	}

	return response, nil
}

func (h *defaultHandler) ProcessSubsequentPreorderRequest(ctx context.Context, request xealth_models.PreorderFormRequest1) (*xealth_models.PreorderFormResponse, error) {
	clinic, err := h.FindMatchingClinic(ctx, request.Deployment)
	if err != nil {
		return nil, err
	} else if clinic == nil {
		return nil, fmt.Errorf("%w: couldn't find matching clinic", errors.NotFound)
	}

	criteria, err := GetPatientMatchingCriteria(request.Datasets, clinic)
	if err != nil {
		return nil, err
	} else if criteria == nil {
		return nil, nil
	}

	matchingPatients, err := h.FindMatchingPatients(ctx, criteria, clinic)
	if err != nil {
		return nil, err
	}

	if count := len(matchingPatients); count != 0 {
		return nil, fmt.Errorf("a matching patient already exists")
	}

	response := &xealth_models.PreorderFormResponse{}
	if criteria.IsPatientUnder13() {
		formData, err := DecodeFormData[GuardianFormData](request.FormData.UserInput)
		if err != nil {
			return nil, err
		}
		errs := formData.Validate()
		if errs != nil {
			formResponse := xealth_models.PreorderFormResponse0{
				DataTrackingId: request.FormData.DataTrackingId,
			}
			if err := PopulateGuardianEnrollmentForm(&formResponse, formData, *errs); err != nil {
				return nil, err
			}
			if err := response.FromPreorderFormResponse0(formResponse); err != nil {
				return nil, err
			}
		}
	} else {
		formData, err := DecodeFormData[PatientFormData](request.FormData.UserInput)
		if err != nil {
			return nil, err
		}
		errs := formData.Validate()
		if errs != nil {
			formResponse := xealth_models.PreorderFormResponse0{
				DataTrackingId: request.FormData.DataTrackingId,
			}
			if err := PopulatePatientEnrollmentForm(&formResponse, formData, *errs); err != nil {
				return nil, err
			}
			if err := response.FromPreorderFormResponse0(formResponse); err != nil {
				return nil, err
			}
		}
	}

	if err := response.FromPreorderFormResponse1(xealth_models.PreorderFormResponse1{}); err != nil {
		return nil, err
	}
	
	return response, nil
}

func (h *defaultHandler) AuthorizeRequest(req *http.Request) error {
	authz := req.Header.Get(authorizationHeader)
	if authz == "" || !strings.HasPrefix(authz, bearerPrefix) {
		return fmt.Errorf("%w: bearer token is required", errors.Unauthorized)
	}
	bearer := strings.TrimPrefix(authz, bearerPrefix)
	if bearer == "" || bearer != h.config.BearerToken {
		return fmt.Errorf("%w: bearer token is invalid", errors.Unauthorized)
	}
	return nil
}

func (h *defaultHandler) FindMatchingClinic(ctx context.Context, deployment string) (*clinics.Clinic, error) {
	enabled := true
	filter := &clinics.Filter{
		EHRProvider: &clinics.EHRProviderXealth,
		EHRSourceId: &deployment,
		EHREnabled:  &enabled,
	}
	page := store.Pagination{
		Offset: 0,
		Limit:  2,
	}

	result, err := h.clinics.List(ctx, filter, page)
	if err != nil {
		return nil, err
	}

	if len(result) > 1 {
		return nil, fmt.Errorf("%w: found multiple clinics matching the deployment", errors.Duplicate)
	} else if len(result) == 0 {
		return nil, nil
	}

	return result[0], nil
}

func (h *defaultHandler) FindMatchingPatients(ctx context.Context, criteria *PatientMatchingCriteria, clinic *clinics.Clinic) ([]*patients.Patient, error) {
	clinicId := clinic.Id.Hex()
	page := store.Pagination{
		Offset: 0,
		Limit:  100,
	}

	filter := patients.Filter{
		ClinicId:  &clinicId,
		Mrn:       &criteria.Mrn,
		BirthDate: &criteria.DateOfBirth,
	}
	result, err := h.patients.List(ctx, &filter, page, nil)
	if err != nil {
		return nil, err
	}

	return result.Patients, nil
}

var _ Xealth = &defaultHandler{}

type PatientMatchingCriteria struct {
	FirstName   string
	LastName    string
	FullName    string
	Mrn         string
	DateOfBirth string
	Email       string
}

func (p *PatientMatchingCriteria) IsPatientUnder13() bool {
	dob, err := time.Parse(types.DateFormat, p.DateOfBirth)
	if err != nil {
		return false
	}
	return dob.AddDate(13, 0, 0).After(time.Now())
}

func GetPatientMatchingCriteria(datasets *xealth_models.GeneralDatasets, clinic *clinics.Clinic) (*PatientMatchingCriteria, error) {
	if clinic.EHRSettings == nil {
		return nil, fmt.Errorf("%w: clinic has no EHR settings", errors.BadRequest)
	}
	if datasets == nil {
		return nil, fmt.Errorf("%w: datasets is required", errors.BadRequest)
	}
	if datasets.DemographicsV1 == nil {
		return nil, fmt.Errorf("%w: demographics is required", errors.BadRequest)
	}
	if datasets.DemographicsV1.Ids == nil || len(*datasets.DemographicsV1.Ids) == 0 {
		return nil, fmt.Errorf("%w: demographics ids are required", errors.BadRequest)
	}

	criteria := &PatientMatchingCriteria{}

	mrnIdType := strings.ToLower(clinic.EHRSettings.GetMrnIDType())
	for _, identifier := range *datasets.DemographicsV1.Ids {
		if identifier.Type != nil && strings.ToLower(*identifier.Type) == mrnIdType && identifier.Id != nil {
			criteria.Mrn = *identifier.Id
			break
		}
	}

	if datasets.DemographicsV1.Name != nil {
		names := make([]string, 0, 2)
		if datasets.DemographicsV1.Name.Given != nil && len(*datasets.DemographicsV1.Name.Given) > 0 {
			criteria.FirstName = strings.Join(*datasets.DemographicsV1.Name.Given, " ")
			names = append(names, criteria.FirstName)
		}
		if datasets.DemographicsV1.Name.Family != nil && len(*datasets.DemographicsV1.Name.Family) > 0 {
			criteria.LastName = strings.Join(*datasets.DemographicsV1.Name.Family, " ")
			names = append(names, criteria.LastName)
		}
		if len(names) > 0 {
			criteria.FullName = strings.Join(names, " ")
		}
	}

	if datasets.DemographicsV1.BirthDate != nil {
		criteria.DateOfBirth = datasets.DemographicsV1.BirthDate.String()
	}

	if datasets.DemographicsV1.Telecom != nil {
		for _, v := range *datasets.DemographicsV1.Telecom {
			if v.System != nil && *v.System == xealth_models.GeneralDatasetsDemographicsV1TelecomSystemEmail && v.Value != nil {
				criteria.Email = *v.Value
			}
		}
	}

	if criteria.Mrn == "" {
		return nil, fmt.Errorf("%w: mrn is missing", errors.BadRequest)
	}
	if criteria.DateOfBirth == "" {
		return nil, fmt.Errorf("%w: date of birth is missing", errors.BadRequest)
	}
	if criteria.FullName == "" {
		return nil, fmt.Errorf("%w: full name is missing", errors.BadRequest)
	}

	return criteria, nil
}

type disabledHandler struct{}

func (d *disabledHandler) AuthorizeRequest(req *http.Request) error {
	return fmt.Errorf("the integration is not enabled")
}

func (d *disabledHandler) ProcessInitialPreorderRequest(ctx context.Context, request xealth_models.PreorderFormRequest0) (*xealth_models.PreorderFormResponse, error) {
	return nil, fmt.Errorf("the integration is not enabled")
}

func (d *disabledHandler) ProcessSubsequentPreorderRequest(ctx context.Context, request xealth_models.PreorderFormRequest1) (*xealth_models.PreorderFormResponse, error) {
	return nil, fmt.Errorf("the integration is not enabled")
}

var _ Xealth = &disabledHandler{}
