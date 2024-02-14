package xealth

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/clinic/clinics"
	errs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/xealth_client"
	"github.com/tidepool-org/platform/auth"
	"github.com/tidepool-org/platform/log"
	"github.com/tidepool-org/platform/log/null"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
	restrictedTokenKey  = "restricted_token"

	eventNewOrder    = "order:new"
	eventCancelOrder = "order:cancel"
)

type ModuleConfig struct {
	Enabled bool `envconfig:"TIDEPOOL_XEALTH_ENABLED"`
}

type Config struct {
	BearerToken            string `envconfig:"TIDEPOOL_XEALTH_BEARER_TOKEN" required:"true"`
	ClientId               string `envconfig:"TIDEPOOL_XEALTH_CLIENT_ID" required:"true"`
	ClientSecret           string `envconfig:"TIDEPOOL_XEALTH_CLIENT_SECRET" required:"true"`
	TokenUrl               string `envconfig:"TIDEPOOL_XEALTH_TOKEN_URL" default:"https://auth-sandbox.xealth.io/oauth2/token"`
	ServerBaseUrl          string `envconfig:"TIDEPOOL_XEALTH_SERVER_BASE_URL" default:"https://api-sandbox.xealth.io/v2"`
	TidepoolApplicationUrl string `envconfig:"TIDEPOOL_APPLICATION_URL" required:"true"`
}

type Xealth interface {
	AuthorizeRequest(req *http.Request) error
	ProcessInitialPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest0) (*xealth_client.PreorderFormResponse, error)
	ProcessSubsequentPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest1) (*xealth_client.PreorderFormResponse, error)
	HandleEventNotification(ctx context.Context, event xealth_client.EventNotification) error
	GetPrograms(ctx context.Context, request xealth_client.GetProgramsRequest) (*xealth_client.GetProgramsResponse, error)
	GetProgramUrl(ctx context.Context, request xealth_client.GetProgramUrlRequest) (*xealth_client.GetProgramUrlResponse, error)
	GetPDFReport(ctx context.Context, request PDFReportRequest) (*PDFReport, error)
}

type defaultHandler struct {
	config *Config

	authClient auth.Client
	client     xealth_client.ClientWithResponsesInterface
	clinics    clinics.Service
	logger     *zap.SugaredLogger
	patients   patients.Service
	store      Store
	users      patients.UserService
}

var _ Xealth = &defaultHandler{}

func NewHandler(authClient auth.Client, clinics clinics.Service, patients patients.Service, users patients.UserService, store Store, logger *zap.SugaredLogger) (Xealth, error) {
	cfg := ModuleConfig{}
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	if !cfg.Enabled {
		return &disabledHandler{}, nil
	}

	clientConfig := &Config{}
	if err := envconfig.Process("", clientConfig); err != nil {
		return nil, err
	}

	client, err := NewClient(clientConfig, logger)
	if err != nil {
		return nil, err
	}

	return &defaultHandler{
		authClient: authClient,
		config:     clientConfig,
		client:     client,
		clinics:    clinics,
		patients:   patients,
		users:      users,
		store:      store,
		logger:     logger,
	}, nil
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

func (d *defaultHandler) ProcessInitialPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest0) (*xealth_client.PreorderFormResponse, error) {
	match, err := NewMatcher[*xealth_client.PreorderFormResponse](d.clinics, d.patients, d.logger).
		FromInitialPreorderForRequest(request).
		DisableErrorOnNoMatchingPatients().
		Match(ctx)
	if err != nil {
		return nil, fmt.Errorf("matching initial preorder request: %w", err)
	}

	dataTrackingId := uuid.NewString()
	if match.Criteria.IsPatientUnder13() {
		return NewGuardianFlowResponseBuilder().
			WithMatchingResult(match).
			WithDataTrackingId(dataTrackingId).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, match.Criteria.FullName).
			WithTags(match.Clinic.PatientTags).
			BuildInitialResponse()
	} else {
		formData := PatientFormData{}
		formData.Patient.Email = match.Criteria.Email

		return NewPatientFlowResponseBuilder().
			WithMatchingResult(match).
			WithDataTrackingId(dataTrackingId).
			WithData(formData).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, match.Criteria.FullName).
			WithTags(match.Clinic.PatientTags).
			BuildInitialResponse()
	}
}

func (d *defaultHandler) ProcessSubsequentPreorderRequest(ctx context.Context, request xealth_client.PreorderFormRequest1) (*xealth_client.PreorderFormResponse, error) {
	match, err := NewMatcher[*xealth_client.PreorderFormResponse](d.clinics, d.patients, d.logger).
		FromSubsequentPreorderForRequest(request).
		DisableErrorOnNoMatchingPatients().
		Match(ctx)
	if err != nil {
		return nil, fmt.Errorf("matching subsequent preorder request: %w", err)
	}

	if match.Criteria.IsPatientUnder13() {
		return NewGuardianFlowResponseBuilder().
			WithMatchingResult(match).
			WithDataTrackingId(request.FormData.DataTrackingId).
			WithUserInput(request.FormData.UserInput).
			WithDataValidator(NewGuardianDataValidator(d.users)).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, match.Criteria.FullName).
			WithTags(match.Clinic.PatientTags).
			PersistPreorderDataOnSuccess(ctx, d.store).
			BuildSubsequentResponse()
	} else {
		return NewPatientFlowResponseBuilder().
			WithMatchingResult(match).
			WithDataTrackingId(request.FormData.DataTrackingId).
			WithUserInput(request.FormData.UserInput).
			WithDataValidator(NewPatientDataValidator(d.users)).
			WithRenderedTitleTemplate(FormTitlePatientNameTemplate, match.Criteria.FullName).
			WithTags(match.Clinic.PatientTags).
			PersistPreorderDataOnSuccess(ctx, d.store).
			BuildSubsequentResponse()
	}
}

func (d *defaultHandler) HandleEventNotification(ctx context.Context, event xealth_client.EventNotification) error {
	eventKey := fmt.Sprintf("%s:%s", event.EventType, event.EventContext)
	if eventKey != eventNewOrder && eventKey != eventCancelOrder {
		d.logger.Infof("ignoring unexpected event %v", eventKey)
		return nil
	}

	match, err := NewMatcher[*xealth_client.EventNotificationResponse](d.clinics, d.patients, d.logger).
		FromEventNotification(event).
		DisableErrorOnNoMatchingClinics().
		DisableErrorOnNoMatchingPatients().
		Match(ctx)
	if err != nil {
		return fmt.Errorf("matching event notification request: %w", err)
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

func (d *defaultHandler) GetPrograms(ctx context.Context, event xealth_client.GetProgramsRequest) (*xealth_client.GetProgramsResponse, error) {
	response := &xealth_client.GetProgramsResponse{}
	if err := response.FromGetProgramsResponse1(xealth_client.GetProgramsResponse1{Present: false}); err != nil {
		return nil, err
	}

	match, err := NewMatcher[*xealth_client.GetProgramsResponse](d.clinics, d.patients, d.logger).
		FromProgramsRequest(event).
		OnNoMatchingPatientsRespondWith(response).
		OnNoMatchingClinicsRespondWith(response).
		Match(ctx)
	if err != nil {
		return nil, fmt.Errorf("matching get programs request: %w", err)
	} else if match.Response != nil {
		return match.Response, nil
	}

	patient := match.Patient

	var subscription *patients.EHRSubscription
	if subs, ok := patient.EHRSubscriptions[patients.SubscriptionXealthReports]; ok {
		subscription = &subs
	}

	if subscription == nil || subscription.Provider != clinics.EHRProviderXealth || !subscription.Active || len(subscription.MatchedMessages) == 0 {
		d.logger.Infow("patient doesn't have an active xealth subscription", "clinicId", patient.ClinicId.Hex(), "patientId", *patient.UserId)
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

	lastMatchedMessage := subscription.MatchedMessages[len(subscription.MatchedMessages)-1]
	order, err := d.store.GetOrder(ctx, lastMatchedMessage.DocumentId.Hex())
	if err != nil {
		d.logger.Errorw("unable to retrieve last matched order", "error", err, "clinicId", patient.ClinicId.Hex(), "patientId", *patient.UserId)
		return nil, err
	}

	lastUpload := GetLastUploadDate(patient)
	programId := GetProgramIdFromOrder(order)
	if programId == nil {
		return nil, fmt.Errorf("programId is required")
	}

	lastViewed, err := d.getLastViewedDate(ctx, event, *programId, *match.Clinic, *match.Patient)
	if err != nil {
		return nil, fmt.Errorf("unable to obtain last viewed date: %w", err)
	}

	programs.Programs[0].Description = GetProgramDescription(lastUpload, lastViewed)
	programs.Programs[0].EnrolledDate = GetProgramEnrollmentDateFromOrder(order)
	programs.Programs[0].HasAlert = IsProgramAlertActive(lastUpload, lastViewed)
	programs.Programs[0].HasStatusView = HasStatusView(patient, subscription)
	programs.Programs[0].ProgramId = GetProgramIdFromOrder(order)
	programs.Programs[0].Title = GetProgramTitle()

	if err := response.FromGetProgramsResponse0(programs); err != nil {
		return nil, err
	}

	return response, nil
}

func (d *defaultHandler) GetProgramUrl(ctx context.Context, event xealth_client.GetProgramUrlRequest) (*xealth_client.GetProgramUrlResponse, error) {
	match, err := NewMatcher[*xealth_client.GetProgramUrlResponse](d.clinics, d.patients, d.logger).
		FromProgramUrlRequest(event).
		Match(ctx)
	if err != nil {
		return nil, fmt.Errorf("matching get program url request: %w", err)
	} else if match.Response != nil {
		return match.Response, nil
	}

	sessionToken, err := d.authClient.ServerSessionToken()
	if err != nil {
		return nil, err
	}
	authCtx := log.NewContextWithLogger(ctx, null.NewLogger())
	authCtx = auth.NewContextWithServerSessionToken(authCtx, sessionToken)
	create := &auth.RestrictedTokenCreate{}

	token, err := d.authClient.CreateUserRestrictedToken(authCtx, *match.Patient.UserId, create)
	if err != nil {
		return nil, err
	}

	url, err := GeneratePDFViewerUrl(d.config.TidepoolApplicationUrl, token.ID, *match.Patient, *match.Clinic)
	if err != nil {
		d.logger.Errorw("unable to generate report url", "clinicId", match.Clinic.Id.Hex(), "patientId", *match.Patient.UserId, "error", err)
		return nil, err
	}

	response := &xealth_client.GetProgramUrlResponse{
		Url: url.String(),
	}

	if err := d.updateLastViewedDate(ctx, event, *match.Clinic, *match.Patient); err != nil {
		d.logger.Errorw(
			"unable to update report last viewed date", "error", err,
			"clinicId", match.Clinic.Id.Hex(),
			"patientId", *match.Patient.UserId,
		)
	}

	return response, nil
}

func (d *defaultHandler) getLastViewedDate(ctx context.Context, event xealth_client.GetProgramsRequest, programId string, clinic clinics.Clinic, patient patients.Patient) (lastViewed time.Time, err error) {
	if event.Datasets == nil || event.Datasets.EhrUserV1 == nil || event.Datasets.EhrUserV1.UserId == nil {
		return
	}

	report, err := d.store.GetMostRecentReportView(ctx, ReportViewFilter{
		ClinicId:      *clinic.Id,
		DeploymentId:  event.Deployment,
		PatientUserId: *patient.UserId,
		ProgramId:     programId,
		UserId:        *event.Datasets.EhrUserV1.UserId,
	})
	if errors.Is(err, errs.NotFound) {
		err = nil
		return
	} else if err != nil {
		return
	}

	lastViewed = report.CreatedTime
	return
}

func (d *defaultHandler) updateLastViewedDate(ctx context.Context, event xealth_client.GetProgramUrlRequest, clinic clinics.Clinic, patient patients.Patient) error {
	if event.Datasets == nil || event.Datasets.EhrUserV1 == nil || event.Datasets.EhrUserV1.UserId == nil {
		return nil
	}

	view := ReportView{
		Id:            nil,
		UserId:        *event.Datasets.EhrUserV1.UserId,
		DeploymentId:  event.Deployment,
		SystemLogin:   event.Datasets.EhrUserV1.SystemLogin,
		PatientUserId: *patient.UserId,
		ProgramId:     event.ProgramId,
		ClinicId:      *clinic.Id,
		CreatedTime:   time.Now(),
	}
	_, err := d.store.CreateReportView(ctx, view)
	if err != nil {
		d.logger.Errorw(
			"unable to update last viewed date",
			"deployment", event.Deployment,
			"view", view,
			"error", err,
		)
	}

	return err
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

	match, err := NewMatcher[*xealth_client.EventNotificationResponse](d.clinics, d.patients, d.logger).
		FromOrder(*order).
		DisableErrorOnNoMatchingClinics().
		DisableErrorOnNoMatchingPatients().
		Match(ctx)
	if err != nil {
		return fmt.Errorf("handling new order: %w", err)
	}

	if match.Clinic == nil {
		d.logger.Errorw("unable to find matching clinic for xealth deployment", "deployment", order.OrderData.OrderInfo.Deployment)
		return nil
	}

	update, err := GetSubscriptionUpdateFromOrderEvent(*order, match.Clinic)
	if err != nil {
		return fmt.Errorf("unable to create subscription update: %w", err)
	}

	if match.Patient == nil {
		if preorderData == nil {
			return fmt.Errorf("%w: preorder data is required to create a new patient", errs.BadRequest)
		}

		validTagIds := make(map[string]struct{})
		for _, tag := range match.Clinic.PatientTags {
			validTagIds[tag.Id.Hex()] = struct{}{}
		}

		create := patients.Patient{
			ClinicId:    match.Clinic.Id,
			BirthDate:   &match.Criteria.DateOfBirth,
			Mrn:         &match.Criteria.Mrn,
			Permissions: &patients.CustodialAccountPermissions,
		}
		connectDexcom := false
		if preorderData.Guardian != nil {
			create.FullName = &match.Criteria.FullName
			if strings.TrimSpace(preorderData.Guardian.Email) != "" {
				create.Email = &preorderData.Guardian.Email
				connectDexcom = preorderData.Guardian.ConnectDexcom
			}
		} else if preorderData.Patient != nil {
			create.FullName = &match.Criteria.FullName
			if strings.TrimSpace(preorderData.Patient.Email) != "" {
				create.Email = &preorderData.Patient.Email
				connectDexcom = preorderData.Patient.ConnectDexcom
			}
		} else {
			return fmt.Errorf("%w: unable to create patient preorder data is missing", errs.BadRequest)
		}

		var tags []primitive.ObjectID
		if preorderData.Tags != nil {
			tags = make([]primitive.ObjectID, 0, len(preorderData.Tags.Ids))
			for _, tagId := range preorderData.Tags.Ids {
				if _, ok := validTagIds[tagId]; ok {
					if objId, err := primitive.ObjectIDFromHex(tagId); err == nil {
						tags = append(tags, objId)
					}
				}
			}
		}

		if len(tags) > 0 {
			create.Tags = &tags
		}

		if connectDexcom {
			dataSources := []patients.DataSource{{
				ProviderName: patients.DexcomDataSourceProviderName,
				State:        patients.DataSourceStatePending,
			}}
			create.DataSources = &dataSources
		}

		match.Patient, err = d.patients.Create(ctx, create)
		if err != nil {
			return fmt.Errorf("unable to create new patient from order: %w", err)
		}

		if connectDexcom {
			if match.Patient, err = d.patients.UpdateLastRequestedDexcomConnectTime(ctx, &patients.LastRequestedDexcomConnectUpdate{
				ClinicId: create.ClinicId.Hex(),
				UserId:   *match.Patient.UserId,
				Time:     time.Now(),
			}); err != nil {
				return fmt.Errorf("unable to update dexcom connection: %w", err)
			}
		}
	}

	err = d.patients.UpdateEHRSubscription(ctx, match.Clinic.Id.Hex(), *match.Patient.UserId, *update)
	if err != nil {
		return fmt.Errorf("unable to update ehr subscription: %w", err)
	}

	return nil
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

type PDFReportRequest struct {
	ClinicId        string
	PatientId       string
	RestrictedToken string
}

type PDFReport struct {
	PdfUrl string
}

func (d *defaultHandler) GetPDFReport(ctx context.Context, request PDFReportRequest) (*PDFReport, error) {
	sessionToken, err := d.authClient.ServerSessionToken()
	if err != nil {
		return nil, err
	}
	authCtx := log.NewContextWithLogger(ctx, null.NewLogger())
	authCtx = auth.NewContextWithServerSessionToken(authCtx, sessionToken)
	token, err := d.authClient.GetRestrictedToken(authCtx, request.RestrictedToken)
	if err != nil {
		return nil, err
	}
	if token == nil || token.ExpirationTime.Before(time.Now()) || token.UserID != request.PatientId {
		return nil, fmt.Errorf("%w: invalid or expired token", errs.Unauthorized)
	}

	clinic, err := d.clinics.Get(ctx, request.ClinicId)
	if err != nil {
		return nil, err
	}
	patient, err := d.patients.Get(ctx, request.ClinicId, request.PatientId)
	if err != nil {
		return nil, err
	}

	if !clinic.EHRSettings.Enabled || getActiveSubscription(*patient) == nil {
		return nil, fmt.Errorf("%w: patient doesn't have active subscriptions", errs.Unauthorized)
	}

	url, err := GenerateReportUrl(d.config.TidepoolApplicationUrl, token.ID, *patient, *clinic)
	if err != nil {
		d.logger.Errorw("unable to generate report url", "clinicId", clinic.Id.Hex(), "error", err)
		return nil, err
	}

	return &PDFReport{PdfUrl: url.String()}, nil
}

func getActiveSubscription(patient patients.Patient) *patients.EHRSubscription {
	if patient.EHRSubscriptions == nil {
		return nil
	}

	subsc, ok := patient.EHRSubscriptions[patients.SubscriptionXealthReports]
	if !ok || !subsc.Active {
		return nil
	}
	return &subsc

}
