package xealth

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/clinic/clinics"
	errs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/xealth_client"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "

	eventNewOrder    = "order:new"
	eventCancelOrder = "order:cancel"
)

type ModuleConfig struct {
	Enabled bool `envconfig:"TIDEPOOL_XEALTH_ENABLED"`
}

type ClientConfig struct {
	BearerToken   string `envconfig:"TIDEPOOL_XEALTH_BEARER_TOKEN" required:"true"`
	ClientId      string `envconfig:"TIDEPOOL_XEALTH_CLIENT_ID" required:"true"`
	ClientSecret  string `envconfig:"TIDEPOOL_XEALTH_CLIENT_SECRET" required:"true"`
	TokenUrl      string `envconfig:"TIDEPOOL_XEALTH_TOKEN_URL" default:"https://auth-sandbox.xealth.io/oauth2/token"`
	ServerBaseUrl string `envconfig:"TIDEPOOL_XEALTH_SERVER_BASE_URL" default:"https://api-sandbox.xealth.io/v2"`
}

type Xealth interface {
	AuthorizeRequest(req *http.Request) error
	ProcessInitialPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest0) (*xealth_client.PreorderFormResponse, error)
	ProcessSubsequentPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest1) (*xealth_client.PreorderFormResponse, error)
	HandleEventNotification(ctx context.Context, event xealth_client.EventNotification) error
	GetPrograms(ctx context.Context, requestrequest xealth_client.GetProgramsRequest) (*xealth_client.GetProgramsResponse, error)
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

	client, err := NewClient(clientConfig, logger)
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
	match, err := NewMatcher[*xealth_client.PreorderFormResponse](d.clinics, d.patients).
		FromInitialPreorderForRequest(request).
		OnNoMatchingClinicsRespondWith(nil, NoClinicsErr).
		OnMultipleMatchingClinicsRespondWith(nil, MultipleClinicsErr).
		OnMultipleMatchingPatientsRespondWith(nil, MultiplePatientsErr).
		Match(ctx)
	if err != nil {
		return nil, err
	}
	if match.Error != nil || match.Response != nil {
		return match.Response, match.Error
	}
	if match.Patient != nil {
		return NewFinalResponse()
	}

	dataTrackingId := uuid.NewString()
	if match.Criteria.IsPatientUnder13() {
		return NewGuardianFlowResponseBuilder().
			WithDataTrackingId(dataTrackingId).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, match.Criteria.FullName).
			BuildInitialResponse()
	} else {
		formData := PatientFormData{}
		formData.Patient.Email = match.Criteria.Email

		return NewPatientFlowResponseBuilder().
			WithDataTrackingId(dataTrackingId).
			WithData(formData).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, match.Criteria.FullName).
			BuildInitialResponse()
	}
}

func (d *defaultHandler) ProcessSubsequentPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest1) (*xealth_client.PreorderFormResponse, error) {
	match, err := NewMatcher[*xealth_client.PreorderFormResponse](d.clinics, d.patients).
		FromSubsequentPreorderForRequest(request).
		OnNoMatchingClinicsRespondWith(nil, NoClinicsErr).
		OnMultipleMatchingClinicsRespondWith(nil, MultipleClinicsErr).
		OnMultipleMatchingPatientsRespondWith(nil, MultiplePatientsErr).
		Match(ctx)
	if err != nil {
		return nil, err
	}
	if match.Error != nil || match.Response != nil {
		return match.Response, match.Error
	}
	if match.Patient != nil {
		return nil, fmt.Errorf("a matching patient already exists")
	}

	if match.Criteria.IsPatientUnder13() {
		return NewGuardianFlowResponseBuilder().
			WithDataTrackingId(request.FormData.DataTrackingId).
			WithUserInput(request.FormData.UserInput).
			WithDataValidator(NewGuardianDataValidator(d.users)).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, match.Criteria.FullName).
			PersistPreorderDataOnSuccess(ctx, d.store).
			BuildSubsequentResponse()
	} else {
		return NewPatientFlowResponseBuilder().
			WithDataTrackingId(request.FormData.DataTrackingId).
			WithUserInput(request.FormData.UserInput).
			WithDataValidator(NewPatientDataValidator(d.users)).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, match.Criteria.FullName).
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
	if eventKey != eventNewOrder && eventKey != eventCancelOrder {
		d.logger.Infof("ignoring unexpected event %v", eventKey)
		return nil
	}

	match, err := NewMatcher[any](d.clinics, d.patients).
		FromEventNotification(event).
		OnNoMatchingClinicsRespondWith(nil, nil).
		OnMultipleMatchingClinicsRespondWith(nil, MultipleClinicsErr).
		OnMultipleMatchingPatientsRespondWith(nil, MultiplePatientsErr).
		Match(ctx)
	if err != nil {
		return err
	}

	if match.Error != nil {
		return match.Error
	}

	if match.Clinic == nil {
		d.logger.Infof("ignoring order for unknown deployment %v", event.Deployment)
		return nil
	}

	if match.Clinic.EHRSettings == nil || match.Clinic.EHRSettings.ProcedureCodes.CreateAccountAndEnableReports == nil || event.ProgramId != *match.Clinic.EHRSettings.ProcedureCodes.CreateAccountAndEnableReports {
		d.logger.Infow("ignoring order with unknown program id", "clinicId", match.Clinic.Id.Hex(), "programId", event.ProgramId)
		return nil
	}

	// Retrieve the full order details from Xealth
	data, err := d.GetXealthOrder(ctx, event.Deployment, event.OrderId)
	if err != nil {
		return err
	}

	// Save the order in the database
	order, err := d.store.CreateOrder(ctx, OrderEvent{
		EventNotification: event,
		OrderData:         *data,
	})
	if err != nil {
		return err
	}

	return d.handleNewOrder(ctx, order.Id.Hex())
}

func (d *defaultHandler) GetProgramUrl(ctx context.Context, event xealth_client.GetProgramUrlRequest) (*xealth_client.GetProgramUrlResponse, error) {
	return nil, nil
}

func (d *defaultHandler) GetPrograms(ctx context.Context, event xealth_client.GetProgramsRequest) (*xealth_client.GetProgramsResponse, error) {
	response := &xealth_client.GetProgramsResponse{}
	if err := response.FromGetProgramsResponse1(xealth_client.GetProgramsResponse1{Present: false}); err != nil {
		return nil, err
	}

	match, err := NewMatcher[*xealth_client.GetProgramsResponse](d.clinics, d.patients).
		FromProgramsRequest(event).
		OnNoMatchingClinicsRespondWith(response, nil).
		OnMultipleMatchingClinicsRespondWith(nil, MultipleClinicsErr).
		OnMultipleMatchingPatientsRespondWith(nil, MultiplePatientsErr).
		Match(ctx)
	if err != nil {
		return nil, err
	}

	if match.Error != nil || match.Response != nil {
		return match.Response, match.Error
	}

	patient := match.Patient
	var subscription *patients.EHRSubscription

	if patient != nil {
		if subs, ok := patient.EHRSubscriptions[patients.SubscriptionXealthReports]; ok {
			subscription = &subs
		}
	}

	if patient == nil || subscription == nil || subscription.Provider != clinics.EHRProviderXealth {
		return response, nil
	}

	response = &xealth_client.GetProgramsResponse{}
	programs := xealth_client.GetProgramsResponse0{
		Present: true,
		Programs: []struct {
			// Description Description of the enrolled program
			Description *string `json:"description,omitempty"`

			// EnrolledDate Date when the patient was enrolled into this program. (Format is YYYY-MM-DD)
			EnrolledDate *string `json:"enrolledDate,omitempty"`

			// HasStatusView Indicates whether or not a subscriber dashboard exists for this patient. Setting this field to false will disable the ability for getProgramUrl request to be made for this program
			HasStatusView *bool `json:"hasStatusView,omitempty"`

			// HasAlert Indicates if new information is available for this patient. If true, Xealth will highlight the program in Monitor view to alert the user
			HasAlert *bool `json:"has_alert,omitempty"`

			// ProgramId Subscriber-defined identifier for the program
			ProgramId *string `json:"programId,omitempty"`

			// Status Patient's current enrollment status in the program
			Status *string `json:"status,omitempty"`

			// Title Title of the enrolled program
			Title *string `json:"title,omitempty"`
		}{{}},
	}

	order, err := d.store.GetOrder(ctx, subscription.MatchedMessages[0].DocumentId.Hex())
	if err != nil {
		return nil, err
	}

	lastUpload := GetLastUploadDate(patient)
	lastViewed, err := d.GetLastViewedDate(ctx)
	if err != nil {
		return nil, err
	}

	programs.Programs[0].Description = GetProgramDescription(lastUpload, lastViewed)
	programs.Programs[0].EnrolledDate = GetProgramEnrollmentDateFromOrder(order)
	programs.Programs[0].HasAlert = IsProgramAlertActive(lastUpload, lastViewed)
	programs.Programs[0].HasStatusView = IsSubscriptionActive(subscription)
	programs.Programs[0].ProgramId = GetProgramIdFromOrder(order)
	programs.Programs[0].Title = GetProgramTitle()

	if err := response.FromGetProgramsResponse0(programs); err != nil {
		return nil, err
	}

	return response, nil
}

func (d *defaultHandler) GetLastViewedDate(ctx context.Context) (time.Time, error) {
	return time.Time{}, nil
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

	match, err := NewMatcher[any](d.clinics, d.patients).
		FromOrder(*order).
		OnMultipleMatchingClinicsRespondWith(nil, MultipleClinicsErr).
		OnMultipleMatchingPatientsRespondWith(nil, MultiplePatientsErr).
		Match(ctx)
	if err != nil {
		return err
	}
	if match.Error != nil {
		return match.Error
	}

	if match.Clinic == nil {
		d.logger.Errorw("unable to find matching clinic for xealth deployment", "deploymentId", order.OrderData.OrderInfo.Deployment)
		return nil
	}

	update, err := GetSubscriptionUpdateFromOrderEvent(*order, match.Clinic)
	if err != nil {
		return err
	}

	if match.Patient == nil {
		if preorderData == nil {
			return fmt.Errorf("%w: preorder data is required to create a new patient", errs.BadRequest)
		}

		create := patients.Patient{
			ClinicId:    match.Clinic.Id,
			BirthDate:   &match.Criteria.DateOfBirth,
			Mrn:         &match.Criteria.Mrn,
			Permissions: &patients.CustodialAccountPermissions,
		}
		if preorderData.Guardian != nil {
			fullName := strings.Join([]string{preorderData.Guardian.FirstName, preorderData.Guardian.LastName}, "")
			if strings.TrimSpace(fullName) == "" {
				return fmt.Errorf("%w: unable to create patient because guardian name is missing", errs.BadRequest)
			}
			create.FullName = &fullName
			create.Email = &preorderData.Guardian.Email
		} else if preorderData.Patient != nil {
			create.FullName = &match.Criteria.FullName
			create.Email = &preorderData.Patient.Email
		} else {
			return fmt.Errorf("%w: unable to create patient preorder data is missing", errs.BadRequest)
		}
		if preorderData.Dexcom.Connect {
			create.LastRequestedDexcomConnectTime = time.Now()
		}

		match.Patient, err = d.patients.Create(ctx, create)
		if err != nil {
			return err
		}
	}

	return d.patients.UpdateEHRSubscription(ctx, match.Clinic.Id.Hex(), *match.Patient.UserId, *update)
}

func GetSubscriptionUpdateFromOrderEvent(orderEvent OrderEvent, clinic *clinics.Clinic) (*patients.SubscriptionUpdate, error) {
	if orderEvent.EventNotification.EventType != xealth_client.EventNotificationEventTypeOrder {
		return nil, fmt.Errorf("%w: unsupported event type %s", errs.BadRequest, orderEvent.EventNotification.EventType)
	}

	programId := GetProgramIdFromOrder(&orderEvent)
	if clinic.EHRSettings.ProcedureCodes.CreateAccountAndEnableReports == nil || programId == nil || *clinic.EHRSettings.ProcedureCodes.CreateAccountAndEnableReports != *programId {
		return nil, fmt.Errorf("%w: unknown program id in order %s", errs.BadRequest, orderEvent.OrderData.OrderInfo.OrderId)
	}

	update := patients.SubscriptionUpdate{
		Name: patients.SubscriptionXealthReports,
		MatchedMessage: patients.MatchedMessage{
			DocumentId: *orderEvent.Id,
			DataModel:  string(orderEvent.EventNotification.EventType),
			EventType:  string(orderEvent.EventNotification.EventContext),
		},
		Provider: clinics.EHRProviderXealth,
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

func (d *defaultHandler) GetXealthOrder(ctx context.Context, deployment, orderId string) (*xealth_client.ReadOrderResponse, error) {
	response, err := d.client.GetPartnerReadOrderDeploymentOrderIdWithResponse(ctx, deployment, orderId, nil)
	if err != nil {
		return nil, err
	} else if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response when retrieving order %v", response.StatusCode())
	}

	return response.JSON200, nil
}

var _ Xealth = &defaultHandler{}

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

func (d *disabledHandler) GetPrograms(ctx context.Context, event xealth_client.GetProgramsRequest) (*xealth_client.GetProgramsResponse, error) {
	return nil, fmt.Errorf("the integration is not enabled")
}

var _ Xealth = &disabledHandler{}
