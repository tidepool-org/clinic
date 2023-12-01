package xealth

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	"github.com/oapi-codegen/runtime/types"
	"github.com/tidepool-org/clinic/clinics"
	errs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/xealth_client"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "

	eventNewOrder = "order:new"
)

type ModuleConfig struct {
	Enabled bool `envconfig:"TIDEPOOL_XEALTH_ENABLED"`
}

type ClientConfig struct {
	BearerToken   string `envconfig:"TIDEPOOL_XEALTH_BEARER_TOKEN" required:"true"`
	ClientId      string `envconfig:"TIDEPOOL_XEALTH_CLIENT_ID" required:"true"`
	ClientSecret  string `envconfig:"TIDEPOOL_XEALTH_CLIENT_SECRET" required:"true"`
	TokenUrl      string `envconfig:"TIDEPOOL_XEALTH_TOKEN_URL" default:"https://auth-sandbox.xealth.io/oauth2/token"`
	ServerBaseUrl string `envconfig:"TIDEPOOL_XEALTH_SERVER_BASE_URL" default:"https://api-sandbox.xealth.io"`
}

type Xealth interface {
	AuthorizeRequest(req *http.Request) error
	ProcessInitialPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest0) (*xealth_client.PreorderFormResponse, error)
	ProcessSubsequentPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest1) (*xealth_client.PreorderFormResponse, error)
	HandleEventNotification(ctx context.Context, event xealth_client.EventNotification) error
}

type defaultHandler struct {
	config *ClientConfig

	client   xealth_client.ClientWithResponsesInterface
	clinics  clinics.Service
	logger   *zap.SugaredLogger
	patients patients.Service
	store    Store
	users    patients.UserService
}

func NewHandler(clinics clinics.Service, patients patients.Service, users patients.UserService, store Store, logger *zap.SugaredLogger) (Xealth, error) {
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

	client, err := NewClient(clientConfig)
	if err != nil {
		return nil, err
	}

	return &defaultHandler{
		config:   clientConfig,
		client:   client,
		clinics:  clinics,
		patients: patients,
		users:    users,
		store:    store,
		logger:   logger,
	}, nil
}

func (d *defaultHandler) ProcessInitialPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest0) (*xealth_client.PreorderFormResponse, error) {
	clinic, err := d.FindMatchingClinic(ctx, request.Deployment)
	if err != nil {
		return nil, err
	}

	criteria, err := GetPatientMatchingCriteria(request.Datasets, clinic)
	if err != nil {
		return nil, err
	}

	matchingPatients, err := d.FindMatchingPatients(ctx, criteria, clinic)
	if err != nil {
		return nil, err
	}

	if count := len(matchingPatients); count == 1 {
		return NewFinalResponse()
	} else if count > 1 {
		return nil, fmt.Errorf("%w: multiple matching patients were found", errs.BadRequest)
	}

	dataTrackingId := uuid.NewString()
	if criteria.IsPatientUnder13() {
		return NewGuardianFlowResponseBuilder().
			WithDataTrackingId(dataTrackingId).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, criteria.FullName).
			BuildInitialResponse()
	} else {
		formData := PatientFormData{}
		formData.Patient.Email = criteria.Email

		return NewPatientFlowResponseBuilder().
			WithDataTrackingId(dataTrackingId).
			WithData(formData).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, criteria.FullName).
			BuildInitialResponse()
	}
}

func (d *defaultHandler) ProcessSubsequentPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest1) (*xealth_client.PreorderFormResponse, error) {
	clinic, err := d.FindMatchingClinic(ctx, request.Deployment)
	if err != nil {
		return nil, err
	}

	criteria, err := GetPatientMatchingCriteria(request.Datasets, clinic)
	if err != nil {
		return nil, err
	}

	matchingPatients, err := d.FindMatchingPatients(ctx, criteria, clinic)
	if err != nil {
		return nil, err
	}

	if count := len(matchingPatients); count != 0 {
		return nil, fmt.Errorf("a matching patient already exists")
	}

	if criteria.IsPatientUnder13() {
		return NewGuardianFlowResponseBuilder().
			WithDataTrackingId(request.FormData.DataTrackingId).
			WithUserInput(request.FormData.UserInput).
			WithDataValidator(NewGuardianDataValidator(d.users)).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, criteria.FullName).
			PersistPreorderDataOnSuccess(ctx, d.store).
			BuildSubsequentResponse()
	} else {
		return NewPatientFlowResponseBuilder().
			WithDataTrackingId(request.FormData.DataTrackingId).
			WithUserInput(request.FormData.UserInput).
			WithDataValidator(NewPatientDataValidator(d.users)).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, criteria.FullName).
			PersistPreorderDataOnSuccess(ctx, d.store).
			BuildSubsequentResponse()
	}
}

func (d *defaultHandler) AuthorizeRequest(req *http.Request) error {
	authz := req.Header.Get(authorizationHeader)
	if authz == "" || !strings.HasPrefix(authz, bearerPrefix) {
		return fmt.Errorf("%w: bearer token is required", errs.Unauthorized)
	}
	bearer := strings.TrimPrefix(authz, bearerPrefix)
	if bearer == "" || bearer != d.config.BearerToken {
		return fmt.Errorf("%w: bearer token is invalid", errs.Unauthorized)
	}
	return nil
}

func (d *defaultHandler) HandleEventNotification(ctx context.Context, event xealth_client.EventNotification) error {
	eventKey := fmt.Sprintf("%s:%s", event.EventType, event.EventContext)
	if eventKey != eventNewOrder {
		d.logger.Infof("ignoring unexpected event %v", eventKey)
		return nil
	}

	clinic, err := d.FindMatchingClinic(ctx, event.Deployment)
	if errors.Is(err, errs.NotFound) {
		d.logger.Infof("ignoring order for unknown deployment %v", event.Deployment)
		return nil
	} else if err != nil {
		return err
	}

	if clinic.EHRSettings == nil || clinic.EHRSettings.ProcedureCodes.CreateAccountAndEnableReports == nil || event.ProgramId != *clinic.EHRSettings.ProcedureCodes.CreateAccountAndEnableReports {
		d.logger.Infow("ignoring order with unknown program id", "clinicId", clinic.Id.Hex(), event.ProgramId)
		return nil
	}

	data, err := d.GetXealthOrder(ctx, event.Deployment, event.OrderId)
	if err != nil {
		return err
	}

	order, err := d.store.CreateOrder(ctx, OrderEvent{OrderData: *data})
	if err != nil {
		return err
	}

	return d.handleNewOrder(ctx, order.Id.Hex())
}

func (d *defaultHandler) handleNewOrder(ctx context.Context, documentId string) error {
	order, err := d.store.GetOrder(ctx, documentId)
	if err != nil {
		return err
	}

	var preorderData *PreorderFormData
	if order.OrderData.Preorder != nil && order.OrderData.Preorder.DataTrackingId != nil {
		preorderData, err = d.store.GetPreorderData(ctx, *order.OrderData.Preorder.DataTrackingId)
		if err != nil {
			return err
		}
	}

	clinic, err := d.FindMatchingClinic(ctx, order.OrderData.OrderInfo.Deployment)
	if errors.Is(err, errs.NotFound) {
		d.logger.Errorw("unable to find matching clinic for xealth deployment", "deploymentId", order.OrderData.OrderInfo.Deployment)
		return nil
	}

	criteria, err := GetPatientMatchingCriteriaFromOrder(*order, clinic)
	if err != nil {
		return err
	}
	update, err := GetSubscriptionUpdateFromOrderEvent(*order, clinic)
	if err != nil {
		return err
	}
	matchingPatients, err := d.FindMatchingPatients(ctx, criteria, clinic)
	if err != nil {
		return err
	}

	count := len(matchingPatients)
	var patient *patients.Patient

	if count == 0 {
		if preorderData == nil {
			return fmt.Errorf("%w: preorder data is required to create a new patient", errs.BadRequest)
		}
		create := patients.Patient{
			ClinicId:    clinic.Id,
			BirthDate:   &criteria.DateOfBirth,
			Email:       nil,
			FullName:    &criteria.FullName,
			Mrn:         &criteria.Mrn,
			Permissions: &patients.CustodialAccountPermissions,
		}
		patient, err = d.patients.Create(ctx, create)
		if err != nil {
			return err
		}
	} else if count == 1 {
		patient = matchingPatients[0]
	} else if count > 1 {
		return fmt.Errorf("%w: multiple matching patients found, cannot fulfill order", errs.BadRequest)
	}

	return d.patients.UpdateEHRSubscription(ctx, clinic.Id.Hex(), *patient.UserId, *update)
}

func GetSubscriptionUpdateFromOrderEvent(orderEvent OrderEvent, clinic *clinics.Clinic) (*patients.SubscriptionUpdate, error) {
	if orderEvent.EventNotification.EventType != xealth_client.EventNotificationEventTypeOrder {
		return nil, fmt.Errorf("%w: unsupported event type %s", errs.BadRequest, orderEvent.EventNotification.EventType)
	}

	programId := GetProgramIdFromOrder(orderEvent)
	if clinic.EHRSettings.ProcedureCodes.CreateAccountAndEnableReports == nil || *clinic.EHRSettings.ProcedureCodes.CreateAccountAndEnableReports != programId {
		return nil, fmt.Errorf("%w: unknown program id %s", errs.BadRequest, programId)
	}

	update := patients.SubscriptionUpdate{
		Name: patients.SummaryAndReportsSubscription,
		MatchedMessage: patients.MatchedMessage{
			DocumentId: *orderEvent.Id,
			DataModel:  string(orderEvent.EventNotification.EventType),
			EventType:  string(orderEvent.EventNotification.EventContext),
		},
		Provider: clinics.EHRProviderRedox,
	}

	if orderEvent.EventNotification.EventContext == xealth_client.EventNotificationEventContextNew {
		update.Active = true
	} else if orderEvent.EventNotification.EventContext == xealth_client.EventNotificationEventContextCancel {
		update.Active = false
	} else {
		return nil, fmt.Errorf("%w: unsupported event context %s", errs.BadRequest, orderEvent.EventNotification.EventContext)
	}

	return &update, nil
}

func GetProgramIdFromOrder(orderEvent OrderEvent) string {
	return orderEvent.OrderData.OrderInfo.ProgramId
}

func GetPatientMatchingCriteriaFromOrder(order OrderEvent, clinic *clinics.Clinic) (*PatientMatchingCriteria, error) {
	if clinic.EHRSettings == nil {
		return nil, fmt.Errorf("%w: clinic has no EHR settings", errs.BadRequest)
	}

	if order.OrderData.Datasets == nil {
		return nil, fmt.Errorf("%w: datasets is required", errs.BadRequest)
	}
	datasets := order.OrderData.Datasets
	if datasets.DemographicsV1 == nil {
		return nil, fmt.Errorf("%w: demographics is required", errs.BadRequest)
	}
	if datasets.DemographicsV1.Ids == nil || len(*datasets.DemographicsV1.Ids) == 0 {
		return nil, fmt.Errorf("%w: demographics ids are required", errs.BadRequest)
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
			criteria.FullName = strings.TrimSpace(strings.Join(names, " "))
		}
	}

	if datasets.DemographicsV1.BirthDate != nil {
		criteria.DateOfBirth = datasets.DemographicsV1.BirthDate.String()
	}

	if datasets.DemographicsV1.Telecom != nil {
		for _, v := range *datasets.DemographicsV1.Telecom {
			if v.System != nil && *v.System == xealth_client.ReadOrderResponseDatasetsDemographicsV1TelecomSystemEmail && v.Value != nil {
				criteria.Email = *v.Value
			}
		}
	}

	return criteria, criteria.Validate()
}

func (d *defaultHandler) GetXealthOrder(ctx context.Context, deployment, orderId string) (*xealth_client.ReadOrderResponse, error) {
	response, err := d.client.GetHsReadOrderDeploymentOrderIdWithResponse(ctx, deployment, orderId, nil)
	if err != nil {
		return nil, err
	} else if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response when retrieving order %v", response.StatusCode())
	}

	return response.JSON200, nil
}

func (d *defaultHandler) FindMatchingClinic(ctx context.Context, deployment string) (*clinics.Clinic, error) {
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

	result, err := d.clinics.List(ctx, filter, page)
	if err != nil {
		return nil, err
	}

	if len(result) > 1 {
		return nil, fmt.Errorf("%w: found multiple clinics matching the deployment", errs.Duplicate)
	} else if len(result) == 0 {
		return nil, fmt.Errorf("%w: couldn't find matching clinic", errs.NotFound)
	}

	return result[0], nil
}

func (d *defaultHandler) FindMatchingPatients(ctx context.Context, criteria *PatientMatchingCriteria, clinic *clinics.Clinic) ([]*patients.Patient, error) {
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
	result, err := d.patients.List(ctx, &filter, page, nil)
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

func (p *PatientMatchingCriteria) Validate() error {
	if p.Mrn == "" {
		return fmt.Errorf("%w: mrn is missing", errs.BadRequest)
	}
	if p.DateOfBirth == "" {
		return fmt.Errorf("%w: date of birth is missing", errs.BadRequest)
	}
	if p.FullName == "" {
		return fmt.Errorf("%w: full name is missing", errs.BadRequest)
	}
	return nil
}

func GetPatientMatchingCriteria(datasets *xealth_client.GeneralDatasets, clinic *clinics.Clinic) (*PatientMatchingCriteria, error) {
	if clinic.EHRSettings == nil {
		return nil, fmt.Errorf("%w: clinic has no EHR settings", errs.BadRequest)
	}
	if datasets == nil {
		return nil, fmt.Errorf("%w: datasets is required", errs.BadRequest)
	}
	if datasets.DemographicsV1 == nil {
		return nil, fmt.Errorf("%w: demographics is required", errs.BadRequest)
	}
	if datasets.DemographicsV1.Ids == nil || len(*datasets.DemographicsV1.Ids) == 0 {
		return nil, fmt.Errorf("%w: demographics ids are required", errs.BadRequest)
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
			criteria.FullName = strings.TrimSpace(strings.Join(names, " "))
		}
	}

	if datasets.DemographicsV1.BirthDate != nil {
		criteria.DateOfBirth = datasets.DemographicsV1.BirthDate.String()
	}

	if datasets.DemographicsV1.Telecom != nil {
		for _, v := range *datasets.DemographicsV1.Telecom {
			if v.System != nil && *v.System == xealth_client.GeneralDatasetsDemographicsV1TelecomSystemEmail && v.Value != nil {
				criteria.Email = *v.Value
			}
		}
	}

	return criteria, criteria.Validate()
}

type disabledHandler struct{}

func (d *disabledHandler) HandleEventNotification(ctx context.Context, event xealth_client.EventNotification) error {
	return fmt.Errorf("the integration is not enabled")
}

func (d *disabledHandler) AuthorizeRequest(req *http.Request) error {
	return fmt.Errorf("the integration is not enabled")
}

func (d *disabledHandler) ProcessInitialPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest0) (*xealth_client.PreorderFormResponse, error) {
	return nil, fmt.Errorf("the integration is not enabled")
}

func (d *disabledHandler) ProcessSubsequentPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest1) (*xealth_client.PreorderFormResponse, error) {
	return nil, fmt.Errorf("the integration is not enabled")
}

var _ Xealth = &disabledHandler{}
