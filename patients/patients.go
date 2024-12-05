package patients

import (
	"context"
	"fmt"
	"time"

	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	SubscriptionRedoxSummaryAndReports = "summaryAndReports"
	SubscriptionXealthReports          = "xealthReports"
)

var (
	ErrNotFound           = fmt.Errorf("patient %w", errors.NotFound)
	ErrPermissionNotFound = fmt.Errorf("permission %w", errors.NotFound)
	ErrDuplicatePatient   = fmt.Errorf("%w: patient is already a member of the clinic", errors.Duplicate)
	ErrDuplicateEmail     = fmt.Errorf("%w: email address is already taken", errors.Duplicate)
	ErrReviewNotOwner     = fmt.Errorf("%w: cannot revert review from another clinician", errors.Conflict)

	PendingDexcomDataSourceExpirationDuration = time.Hour * 24 * 30
	DexcomDataSourceProviderName              = "dexcom"
	DataSourceStatePending                    = "pending"
	DataSourceStatePendingReconnect           = "pendingReconnect"

	permission                  = make(Permission, 0)
	CustodialAccountPermissions = Permissions{
		Custodian: &permission,
		View:      &permission,
		Upload:    &permission,
		Note:      &permission,
	}

	precedingScheduledOrderPeriod = time.Minute * 5
)

//go:generate mockgen --build_flags=--mod=mod -source=./patients.go -destination=./test/mock_service.go -package test MockService
type Service interface {
	Get(ctx context.Context, clinicId string, userId string) (*Patient, error)
	Count(ctx context.Context, filter *Filter) (int, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination, sort []*store.Sort) (*ListResult, error)
	Create(ctx context.Context, patient Patient) (*Patient, error)
	Update(ctx context.Context, update PatientUpdate) (*Patient, error)
	AddReview(ctx context.Context, clinicId, userId string, review Review) ([]Review, error)
	DeleteReview(ctx context.Context, clinicId, clinicianId, userId string) ([]Review, error)
	UpdateEmail(ctx context.Context, userId string, email *string) error
	Remove(ctx context.Context, clinicId string, userId string, deletedByUserId *string) error
	UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *Permissions) (*Patient, error)
	DeletePermission(ctx context.Context, clinicId, userId, permission string) (*Patient, error)
	DeleteFromAllClinics(ctx context.Context, userId string) ([]string, error)
	DeleteNonCustodialPatientsOfClinic(ctx context.Context, clinicId string) (bool, error)
	UpdateSummaryInAllClinics(ctx context.Context, userId string, summary *Summary) error
	UpdateLastUploadReminderTime(ctx context.Context, update *UploadReminderUpdate) (*Patient, error)
	UpdateLastRequestedDexcomConnectTime(ctx context.Context, update *LastRequestedDexcomConnectUpdate) (*Patient, error)
	AssignPatientTagToClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error
	DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error
	UpdatePatientDataSources(ctx context.Context, userId string, dataSources *DataSources) error
	TideReport(ctx context.Context, clinicId string, params TideReportParams) (*Tide, error)
	UpdateEHRSubscription(ctx context.Context, clinicId, userId string, update SubscriptionUpdate) error
	RescheduleLastSubscriptionOrderForAllPatients(ctx context.Context, clinicId, subscription, ordersCollection, targetCollection string) error
	RescheduleLastSubscriptionOrderForPatient(ctx context.Context, clinicIds []string, userId, subscription, ordersCollection, targetCollection string) error
}

type Patient struct {
	Id                             *primitive.ObjectID   `bson:"_id,omitempty"`
	ClinicId                       *primitive.ObjectID   `bson:"clinicId,omitempty"`
	UserId                         *string               `bson:"userId,omitempty"`
	BirthDate                      *string               `bson:"birthDate"`
	Email                          *string               `bson:"email"`
	FullName                       *string               `bson:"fullName"`
	Mrn                            *string               `bson:"mrn"`
	TargetDevices                  *[]string             `bson:"targetDevices"`
	Tags                           *[]primitive.ObjectID `bson:"tags,omitempty"`
	DataSources                    *[]DataSource         `bson:"dataSources,omitempty"`
	Permissions                    *Permissions          `bson:"permissions,omitempty"`
	IsMigrated                     bool                  `bson:"isMigrated,omitempty"`
	LegacyClinicianIds             []string              `bson:"legacyClinicianIds,omitempty"`
	CreatedTime                    time.Time             `bson:"createdTime,omitempty"`
	UpdatedTime                    time.Time             `bson:"updatedTime,omitempty"`
	InvitedBy                      *string               `bson:"invitedBy,omitempty"`
	Summary                        *Summary              `bson:"summary,omitempty"`
	Reviews                        []Review              `bson:"reviews,omitempty"`
	LastUploadReminderTime         time.Time             `bson:"lastUploadReminderTime,omitempty"`
	LastRequestedDexcomConnectTime time.Time             `bson:"lastRequestedDexcomConnectTime,omitempty"`
	RequireUniqueMrn               bool                  `bson:"requireUniqueMrn"`
	EHRSubscriptions               EHRSubscriptions      `bson:"ehrSubscriptions,omitempty"`
}

func (p Patient) IsCustodial() bool {
	return p.Permissions != nil && p.Permissions.Custodian != nil
}

type Deletion struct {
	Id              *primitive.ObjectID `bson:"_id,omitempty"`
	Patient         Patient             `bson:"patient"`
	DeletedTime     time.Time           `bson:"deletedTime,omitempty"`
	DeletedByUserId *string             `bson:"deletedByUserId,omitempty"`
}

type EHRSubscriptions map[string]EHRSubscription

type EHRSubscription struct {
	Active          bool             `bson:"active"`
	Provider        string           `bson:"provider"`
	MatchedMessages []MatchedMessage `bson:"matchedMessages,omitempty"`
	CreatedAt       time.Time        `bson:"createdAt"`
	UpdatedAt       time.Time        `bson:"updatedAt"`
}

type MatchedMessage struct {
	DocumentId primitive.ObjectID `bson:"id"`
	DataModel  string             `bson:"dataModel"`
	EventType  string             `bson:"eventType"`
}

type Review struct {
	ClinicianId string    `json:"clinicianId"`
	Time        time.Time `json:"time"`
}

type SubscriptionUpdate struct {
	Name           string
	Provider       string
	Active         bool
	MatchedMessage MatchedMessage
}

type FilterPair struct {
	Cmp   string
	Value float64
}

type FilterDatePair struct {
	Min *time.Time
	Max *time.Time
}

type SummaryFilters map[string]FilterPair

type SummaryDateFilters map[string]FilterDatePair

type Filter struct {
	ClinicIds    []string
	ClinicId     *string
	UserId       *string
	Search       *string
	Tags         *[]string
	Mrn          *string
	BirthDate    *string
	FullName     *string
	LastReviewed *time.Time

	HasSubscription *bool
	HasMRN          *bool

	Period *string

	CGM SummaryFilters
	BGM SummaryFilters

	CGMTime SummaryDateFilters
	BGMTime SummaryDateFilters

	ExcludeDemo bool
}

type Permission = map[string]interface{}
type Permissions struct {
	Custodian *Permission `bson:"custodian,omitempty"`
	View      *Permission `bson:"view,omitempty"`
	Upload    *Permission `bson:"upload,omitempty"`
	Note      *Permission `bson:"note,omitempty"`
}

func (p *Permissions) Empty() bool {
	return p.Custodian == nil &&
		p.View == nil &&
		p.Upload == nil &&
		p.Note == nil
}

type ListResult struct {
	Patients   []*Patient `bson:"data"`
	TotalCount int        `bson:"count"`
}

type PatientUpdate struct {
	ClinicId  string
	UserId    string
	Patient   Patient
}

type UploadReminderUpdate struct {
	ClinicId  string
	UserId    string
	UpdatedBy string
	Time      time.Time
}

type LastRequestedDexcomConnectUpdate struct {
	ClinicId string
	UserId   string
	Time     time.Time
}

type Summary struct {
	CGM *PatientCGMStats `json:"cgmStats" bson:"cgmStats"`
	BGM *PatientBGMStats `json:"bgmStats" bson:"bgmStats"`
}

func (s *Summary) GetLastUploadDate() time.Time {
	last := time.Time{}
	if s.CGM != nil && s.CGM.GetLastUploadDate().After(last) {
		last = s.CGM.GetLastUploadDate()
	}
	if s.BGM != nil && s.BGM.GetLastUploadDate().After(last) {
		last = s.BGM.GetLastUploadDate()
	}
	return last
}

func (s *Summary) GetLastUpdatedDate() time.Time {
	last := time.Time{}
	if s.CGM != nil && s.CGM.GetLastUpdatedDate().After(last) {
		last = s.CGM.GetLastUpdatedDate()
	}
	if s.BGM != nil && s.BGM.GetLastUpdatedDate().After(last) {
		last = s.BGM.GetLastUpdatedDate()
	}
	return last
}

type DataSources []DataSource
type DataSource struct {
	DataSourceId   *primitive.ObjectID `bson:"dataSourceId,omitempty"`
	ModifiedTime   *time.Time          `bson:"modifiedTime,omitempty"`
	ExpirationTime *time.Time          `bson:"expirationTime,omitempty"`
	ProviderName   string              `bson:"providerName"`
	State          string              `bson:"state"`
}

type TideReportParams struct {
	Period         *string
	Tags           *[]string
	LastDataCutoff *time.Time
}
