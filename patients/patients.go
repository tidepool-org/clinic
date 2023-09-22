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
	SummaryAndReportsSubscription = "summaryAndReports"
)

var (
	ErrNotFound           = fmt.Errorf("patient %w", errors.NotFound)
	ErrPermissionNotFound = fmt.Errorf("permission %w", errors.NotFound)
	ErrDuplicatePatient   = fmt.Errorf("%w: patient is already a member of the clinic", errors.Duplicate)
	ErrDuplicateEmail     = fmt.Errorf("%w: email address is already taken", errors.Duplicate)

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
)

//go:generate mockgen --build_flags=--mod=mod -source=./patients.go -destination=./test/mock_service.go -package test MockService
type Service interface {
	Get(ctx context.Context, clinicId string, userId string) (*Patient, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination, sort []*store.Sort) (*ListResult, error)
	Create(ctx context.Context, patient Patient) (*Patient, error)
	Update(ctx context.Context, update PatientUpdate) (*Patient, error)
	UpdateEmail(ctx context.Context, userId string, email *string) error
	Remove(ctx context.Context, clinicId string, userId string) error
	UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *Permissions) (*Patient, error)
	DeletePermission(ctx context.Context, clinicId, userId, permission string) (*Patient, error)
	DeleteFromAllClinics(ctx context.Context, userId string) error
	DeleteNonCustodialPatientsOfClinic(ctx context.Context, clinicId string) error
	UpdateSummaryInAllClinics(ctx context.Context, userId string, summary *Summary) error
	UpdateLastUploadReminderTime(ctx context.Context, update *UploadReminderUpdate) (*Patient, error)
	UpdateLastRequestedDexcomConnectTime(ctx context.Context, update *LastRequestedDexcomConnectUpdate) (*Patient, error)
	AssignPatientTagToClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error
	DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error
	UpdatePatientDataSources(ctx context.Context, userId string, dataSources *DataSources) error
	UpdateEHRSubscription(ctx context.Context, clinicId, userId string, update SubscriptionUpdate) error
	RescheduleLastSubscriptionOrderForAllPatients(ctx context.Context, clinicId, subscription, ordersCollection, targetCollection string) error
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
	LastUploadReminderTime         time.Time             `bson:"lastUploadReminderTime,omitempty"`
	LastRequestedDexcomConnectTime time.Time             `bson:"lastRequestedDexcomConnectTime,omitempty"`
	RequireUniqueMrn               bool                  `bson:"requireUniqueMrn"`
	EHRSubscriptions               EHRSubscriptions      `bson:"ehrSubscriptions,omitempty"`
}

func (p Patient) IsCustodial() bool {
	return p.Permissions != nil && p.Permissions.Custodian != nil
}

type EHRSubscriptions map[string]EHRSubscription

type EHRSubscription struct {
	Active          bool             `bson:"active"`
	MatchedMessages []MatchedMessage `bson:"matchedMessages,omitempty"`
}

type MatchedMessage struct {
	DocumentId primitive.ObjectID `bson:"id"`
	DataModel  string             `bson:"dataModel"`
	EventType  string             `bson:"eventType"`
}

type SubscriptionUpdate struct {
	Name           string
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
	ClinicId  *string
	UserId    *string
	Search    *string
	Tags      *[]string
	Mrn       *string
	BirthDate *string
	FullName  *string

	ActiveEHRSubscription *string

	Period *string

	CGM SummaryFilters
	BGM SummaryFilters

	CGMTime SummaryDateFilters
	BGMTime SummaryDateFilters
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
	UpdatedBy string
}

type UploadReminderUpdate struct {
	ClinicId  string
	UserId    string
	UpdatedBy string
	Time      time.Time
}

type LastRequestedDexcomConnectUpdate struct {
	ClinicId  string
	UserId    string
	UpdatedBy string
	Time      time.Time
}

type CGMPeriod struct {
	HasAverageGlucose *bool           `json:"hasAverageGlucose" bson:"hasAverageGlucose"`
	AverageGlucose    *AverageGlucose `json:"averageGlucose" bson:"averageGlucose"`

	HasGlucoseManagementIndicator *bool    `json:"hasGlucoseManagementIndicator" bson:"hasGlucoseManagementIndicator"`
	GlucoseManagementIndicator    *float64 `json:"glucoseManagementIndicator" bson:"glucoseManagementIndicator"`

	HasTotalRecords *bool `json:"hasTotalRecords" bson:"hasTotalRecords"`
	TotalRecords    *int  `json:"totalRecords" bson:"totalRecords"`

	HasAverageDailyRecords *bool    `json:"hasAverageDailyRecords" bson:"hasAverageDailyRecords"`
	AverageDailyRecords    *float64 `json:"averageDailyRecords" bson:"averageDailyRecords"`

	HasTimeCGMUsePercent *bool    `json:"hasTimeCGMUsePercent" bson:"hasTimeCGMUsePercent"`
	TimeCGMUsePercent    *float64 `json:"timeCGMUsePercent" bson:"timeCGMUsePercent"`

	HasTimeCGMUseMinutes *bool `json:"hasTimeCGMUseMinutes" bson:"hasTimeCGMUseMinutes"`
	TimeCGMUseMinutes    *int  `json:"timeCGMUseMinutes" bson:"timeCGMUseMinutes"`

	HasTimeCGMUseRecords *bool `json:"hasTimeCGMUseRecords" bson:"hasTimeCGMUseRecords"`
	TimeCGMUseRecords    *int  `json:"timeCGMUseRecords" bson:"timeCGMUseRecords"`

	HasTimeInTargetPercent *bool    `json:"hasTimeInTargetPercent" bson:"hasTimeInTargetPercent"`
	TimeInTargetPercent    *float64 `json:"timeInTargetPercent" bson:"timeInTargetPercent"`

	HasTimeInTargetMinutes *bool `json:"hasTimeInTargetMinutes" bson:"hasTimeInTargetMinutes"`
	TimeInTargetMinutes    *int  `json:"timeInTargetMinutes" bson:"timeInTargetMinutes"`

	HasTimeInTargetRecords *bool `json:"hasTimeInTargetRecords" bson:"hasTimeTimeInTargetRecords"`
	TimeInTargetRecords    *int  `json:"timeInTargetRecords" bson:"timeInTargetRecords"`

	HasTimeInLowPercent *bool    `json:"hasTimeInLowPercent" bson:"hasTimeInLowPercent"`
	TimeInLowPercent    *float64 `json:"timeInLowPercent" bson:"timeInLowPercent"`

	HasTimeInLowMinutes *bool `json:"hasTimeInLowMinutes" bson:"hasTimeInLowMinutes"`
	TimeInLowMinutes    *int  `json:"timeInLowMinutes" bson:"timeInLowMinutes"`

	HasTimeInLowRecords *bool `json:"hasTimeInLowRecords" bson:"hasTimeInLowRecords"`
	TimeInLowRecords    *int  `json:"timeInLowRecords" bson:"timeInLowRecords"`

	HasTimeInVeryLowPercent *bool    `json:"hasTimeInVeryLowPercent" bson:"hasTimeInVeryLowPercent"`
	TimeInVeryLowPercent    *float64 `json:"timeInVeryLowPercent" bson:"timeInVeryLowPercent"`

	HasTimeInVeryLowMinutes *bool `json:"hasTimeInVeryLowMinutes" bson:"hasTimeInVeryLowMinutes"`
	TimeInVeryLowMinutes    *int  `json:"timeInVeryLowMinutes" bson:"timeInVeryLowMinutes"`

	HasTimeInVeryLowRecords *bool `json:"hasTimeInVeryLowRecords" bson:"hasTimeInVeryLowRecords"`
	TimeInVeryLowRecords    *int  `json:"timeInVeryLowRecords" bson:"timeInVeryLowRecords"`

	HasTimeInHighPercent *bool    `json:"hasTimeInHighPercent" bson:"hasTimeInHighPercent"`
	TimeInHighPercent    *float64 `json:"timeInHighPercent" bson:"timeInHighPercent"`

	HasTimeInHighMinutes *bool `json:"hasTimeInHighMinutes" bson:"hasTimeInHighMinutes"`
	TimeInHighMinutes    *int  `json:"timeInHighMinutes" bson:"timeInHighMinutes"`

	HasTimeInHighRecords *bool `json:"hasTimeInHighRecords" bson:"hasTimeInHighRecords"`
	TimeInHighRecords    *int  `json:"timeInHighRecords" bson:"timeInHighRecords"`

	HasTimeInVeryHighPercent *bool    `json:"hasTimeInVeryHighPercent" bson:"hasTimeInVeryHighPercent"`
	TimeInVeryHighPercent    *float64 `json:"timeInVeryHighPercent" bson:"timeInVeryHighPercent"`

	HasTimeInVeryHighMinutes *bool `json:"hasTimeInVeryHighMinutes" bson:"hasTimeInVeryHighMinutes"`
	TimeInVeryHighMinutes    *int  `json:"timeInVeryHighMinutes" bson:"timeInVeryHighMinutes"`

	HasTimeInVeryHighRecords *bool `json:"hasTimeInVeryHighRecords" bson:"hasTimeInVeryHighRecords"`
	TimeInVeryHighRecords    *int  `json:"timeInVeryHighRecords" bson:"timeInVeryHighRecords"`
}

type CGMStats struct {
	Config     *Config               `json:"config" bson:"config"`
	Dates      *Dates                `json:"dates" bson:"dates"`
	Periods    map[string]*CGMPeriod `json:"periods" bson:"periods"`
	TotalHours *int                  `json:"totalHours" bson:"totalHours"`
}

type BGMPeriod struct {
	HasAverageGlucose *bool           `json:"hasAverageGlucose" bson:"hasAverageGlucose"`
	AverageGlucose    *AverageGlucose `json:"averageGlucose" bson:"averageGlucose"`

	HasTotalRecords *bool `json:"hasTotalRecords" bson:"hasTotalRecords"`
	TotalRecords    *int  `json:"totalRecords" bson:"totalRecords"`

	HasAverageDailyRecords *bool    `json:"hasAverageDailyRecords" bson:"hasAverageDailyRecords"`
	AverageDailyRecords    *float64 `json:"averageDailyRecords" bson:"averageDailyRecords"`

	HasTimeInTargetPercent *bool    `json:"hasTimeInTargetPercent" bson:"hasTimeInTargetPercent"`
	TimeInTargetPercent    *float64 `json:"timeInTargetPercent" bson:"timeInTargetPercent"`

	HasTimeInTargetRecords *bool `json:"hasTimeInTargetRecords" bson:"hasTimeTimeInTargetRecords"`
	TimeInTargetRecords    *int  `json:"timeInTargetRecords" bson:"timeInTargetRecords"`

	HasTimeInLowPercent *bool    `json:"hasTimeInLowPercent" bson:"hasTimeInLowPercent"`
	TimeInLowPercent    *float64 `json:"timeInLowPercent" bson:"timeInLowPercent"`

	HasTimeInLowRecords *bool `json:"hasTimeInLowRecords" bson:"hasTimeInLowRecords"`
	TimeInLowRecords    *int  `json:"timeInLowRecords" bson:"timeInLowRecords"`

	HasTimeInVeryLowPercent *bool    `json:"hasTimeInVeryLowPercent" bson:"hasTimeInVeryLowPercent"`
	TimeInVeryLowPercent    *float64 `json:"timeInVeryLowPercent" bson:"timeInVeryLowPercent"`

	HasTimeInVeryLowRecords *bool `json:"hasTimeInVeryLowRecords" bson:"hasTimeInVeryLowRecords"`
	TimeInVeryLowRecords    *int  `json:"timeInVeryLowRecords" bson:"timeInVeryLowRecords"`

	HasTimeInHighPercent *bool    `json:"hasTimeInHighPercent" bson:"hasTimeInHighPercent"`
	TimeInHighPercent    *float64 `json:"timeInHighPercent" bson:"timeInHighPercent"`

	HasTimeInHighRecords *bool `json:"hasTimeInHighRecords" bson:"hasTimeInHighRecords"`
	TimeInHighRecords    *int  `json:"timeInHighRecords" bson:"timeInHighRecords"`

	HasTimeInVeryHighPercent *bool    `json:"hasTimeInVeryHighPercent" bson:"hasTimeInVeryHighPercent"`
	TimeInVeryHighPercent    *float64 `json:"timeInVeryHighPercent" bson:"timeInVeryHighPercent"`

	HasTimeInVeryHighRecords *bool `json:"hasTimeInVeryHighRecords" bson:"hasTimeInVeryHighRecords"`
	TimeInVeryHighRecords    *int  `json:"timeInVeryHighRecords" bson:"timeInVeryHighRecords"`
}

type BGMStats struct {
	Config     *Config               `json:"config" bson:"config"`
	Dates      *Dates                `json:"dates" bson:"dates"`
	Periods    map[string]*BGMPeriod `json:"periods" bson:"periods"`
	TotalHours *int                  `json:"totalHours" bson:"totalHours"`
}

type Config struct {
	SchemaVersion *int `json:"schemaVersion" bson:"schemaVersion"`

	// these are just constants right now.
	HighGlucoseThreshold     *float64 `json:"highGlucoseThreshold" bson:"highGlucoseThreshold"`
	VeryHighGlucoseThreshold *float64 `json:"veryHighGlucoseThreshold" bson:"veryHighGlucoseThreshold"`
	LowGlucoseThreshold      *float64 `json:"lowGlucoseThreshold" bson:"lowGlucoseThreshold"`
	VeryLowGlucoseThreshold  *float64 `json:"VeryLowGlucoseThreshold" bson:"VeryLowGlucoseThreshold"`
}

type Dates struct {
	LastUpdatedDate *time.Time `json:"lastUpdatedDate" bson:"lastUpdatedDate"`

	HasLastUploadDate *bool      `json:"hasLastUploadDate" bson:"hasLastUploadDate"`
	LastUploadDate    *time.Time `json:"lastUploadDate" bson:"lastUploadDate"`

	HasFirstData *bool      `json:"hasFirstData" bson:"hasFirstData"`
	FirstData    *time.Time `json:"firstData" bson:"firstData"`

	HasLastData *bool      `json:"hasLastData" bson:"hasLastData"`
	LastData    *time.Time `json:"lastData" bson:"lastData"`

	HasOutdatedSince *bool      `json:"hasOutdatedSince" bson:"hasOutdatedSince"`
	OutdatedSince    *time.Time `json:"outdatedSince" bson:"outdatedSince"`
}

type Summary struct {
	CGM *CGMStats `json:"cgmStats" bson:"cgmStats"`
	BGM *BGMStats `json:"bgmStats" bson:"bgmStats"`
}

type AverageGlucose struct {
	Units string  `bson:"units"`
	Value float64 `bson:"value"`
}

type DataSources []DataSource
type DataSource struct {
	DataSourceId   *primitive.ObjectID `bson:"dataSourceId,omitempty"`
	ModifiedTime   *time.Time          `bson:"modifiedTime,omitempty"`
	ExpirationTime *time.Time          `bson:"expirationTime,omitempty"`
	ProviderName   string              `bson:"providerName"`
	State          string              `bson:"state"`
}
