package patients

import (
	"context"
	"fmt"
	"time"

	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string) error
	UpdatePatientDataSources(ctx context.Context, userId string, dataSources *DataSources) error
}

type Patient struct {
	Id                             *primitive.ObjectID   `bson:"_id,omitempty"`
	ClinicId                       *primitive.ObjectID   `bson:"clinicId,omitempty"`
	UserId                         *string               `bson:"userId,omitempty"`
	BirthDate                      *string               `bson:"birthDate"`
	Email                          *string               `bson:"email"`
	FullName                       *string               `bson:"fullName"`
	Mrn                            *string               `bson:"mrn"`
	Tags                           *[]primitive.ObjectID `bson:"tags,omitempty"`
	DataSources                    *[]DataSource         `bson:"dataSources,omitempty"`
	TargetDevices                  *[]string             `bson:"targetDevices"`
	Permissions                    *Permissions          `bson:"permissions,omitempty"`
	IsMigrated                     bool                  `bson:"isMigrated,omitempty"`
	LegacyClinicianIds             []string              `bson:"legacyClinicianIds,omitempty"`
	CreatedTime                    time.Time             `bson:"createdTime,omitempty"`
	UpdatedTime                    time.Time             `bson:"updatedTime,omitempty"`
	InvitedBy                      *string               `bson:"invitedBy,omitempty"`
	Summary                        *Summary              `bson:"summary,omitempty"`
	LastUploadReminderTime         time.Time             `bson:"lastUploadReminderTime,omitempty"`
	LastRequestedDexcomConnectTime time.Time             `bson:"lastRequestedDexcomConnectTime,omitempty"`
}

// PatientSummary defines model for PatientSummary.

func (p Patient) IsCustodial() bool {
	return p.Permissions != nil && p.Permissions.Custodian != nil
}

type Filter struct {
	ClinicId *string
	UserId   *string
	Search   *string

	Tags *[]string

	CgmLastUploadDateFrom *time.Time
	CgmLastUploadDateTo   *time.Time

	BgmLastUploadDateFrom *time.Time
	BgmLastUploadDateTo   *time.Time

	Period *string

	CgmTimeCGMUsePercentCmp       *string
	CgmTimeCGMUsePercentValue     float64
	CgmTimeInVeryLowPercentCmp    *string
	CgmTimeInVeryLowPercentValue  float64
	CgmTimeInLowPercentCmp        *string
	CgmTimeInLowPercentValue      float64
	CgmTimeInTargetPercentCmp     *string
	CgmTimeInTargetPercentValue   float64
	CgmTimeInHighPercentCmp       *string
	CgmTimeInHighPercentValue     float64
	CgmTimeInVeryHighPercentCmp   *string
	CgmTimeInVeryHighPercentValue float64

	BgmTimeInVeryLowPercentCmp    *string
	BgmTimeInVeryLowPercentValue  float64
	BgmTimeInLowPercentCmp        *string
	BgmTimeInLowPercentValue      float64
	BgmTimeInTargetPercentCmp     *string
	BgmTimeInTargetPercentValue   float64
	BgmTimeInHighPercentCmp       *string
	BgmTimeInHighPercentValue     float64
	BgmTimeInVeryHighPercentCmp   *string
	BgmTimeInVeryHighPercentValue float64
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
	HasAverageGlucose             *bool `json:"hasAverageGlucose" bson:"hasAverageGlucose"`
	HasGlucoseManagementIndicator *bool `json:"hasGlucoseManagementIndicator" bson:"hasGlucoseManagementIndicator"`
	HasTimeCGMUsePercent          *bool `json:"hasTimeCGMUsePercent" bson:"hasTimeCGMUsePercent"`
	HasTimeInTargetPercent        *bool `json:"hasTimeInTargetPercent" bson:"hasTimeInTargetPercent"`
	HasTimeInHighPercent          *bool `json:"hasTimeInHighPercent" bson:"hasTimeInHighPercent"`
	HasTimeInVeryHighPercent      *bool `json:"hasTimeInVeryHighPercent" bson:"hasTimeInVeryHighPercent"`
	HasTimeInLowPercent           *bool `json:"hasTimeInLowPercent" bson:"hasTimeInLowPercent"`
	HasTimeInVeryLowPercent       *bool `json:"hasTimeInVeryLowPercent" bson:"hasTimeInVeryLowPercent"`

	// actual values
	TimeCGMUsePercent *float64 `json:"timeCGMUsePercent" bson:"timeCGMUsePercent"`
	TimeCGMUseMinutes *int     `json:"timeCGMUseMinutes" bson:"timeCGMUseMinutes"`
	TimeCGMUseRecords *int     `json:"timeCGMUseRecords" bson:"timeCGMUseRecords"`

	AverageGlucose             *AverageGlucose `json:"averageGlucose" bson:"averageGlucose"`
	GlucoseManagementIndicator *float64        `json:"glucoseManagementIndicator" bson:"glucoseManagementIndicator"`

	TotalRecords        *int     `json:"totalRecords" bson:"totalRecords"`
	AverageDailyRecords *float64 `json:"averageDailyRecords" bson:"averageDailyRecords"`

	TimeInTargetPercent *float64 `json:"timeInTargetPercent" bson:"timeInTargetPercent"`
	TimeInTargetMinutes *int     `json:"timeInTargetMinutes" bson:"timeInTargetMinutes"`
	TimeInTargetRecords *int     `json:"timeInTargetRecords" bson:"timeInTargetRecords"`

	TimeInLowPercent *float64 `json:"timeInLowPercent" bson:"timeInLowPercent"`
	TimeInLowMinutes *int     `json:"timeInLowMinutes" bson:"timeInLowMinutes"`
	TimeInLowRecords *int     `json:"timeInLowRecords" bson:"timeInLowRecords"`

	TimeInVeryLowPercent *float64 `json:"timeInVeryLowPercent" bson:"timeInVeryLowPercent"`
	TimeInVeryLowMinutes *int     `json:"timeInVeryLowMinutes" bson:"timeInVeryLowMinutes"`
	TimeInVeryLowRecords *int     `json:"timeInVeryLowRecords" bson:"timeInVeryLowRecords"`

	TimeInHighPercent *float64 `json:"timeInHighPercent" bson:"timeInHighPercent"`
	TimeInHighMinutes *int     `json:"timeInHighMinutes" bson:"timeInHighMinutes"`
	TimeInHighRecords *int     `json:"timeInHighRecords" bson:"timeInHighRecords"`

	TimeInVeryHighPercent *float64 `json:"timeInVeryHighPercent" bson:"timeInVeryHighPercent"`
	TimeInVeryHighMinutes *int     `json:"timeInVeryHighMinutes" bson:"timeInVeryHighMinutes"`
	TimeInVeryHighRecords *int     `json:"timeInVeryHighRecords" bson:"timeInVeryHighRecords"`
}

type CGMStats struct {
	Config     *Config               `json:"config" bson:"config"`
	Dates      *Dates                `json:"dates" bson:"dates"`
	Periods    map[string]*CGMPeriod `json:"periods" bson:"periods"`
	TotalHours *int                  `json:"totalHours" bson:"totalHours"`
}

type BGMPeriod struct {
	HasAverageGlucose        *bool `json:"hasAverageGlucose" bson:"hasAverageGlucose"`
	HasTimeInTargetPercent   *bool `json:"hasTimeInTargetPercent" bson:"hasTimeInTargetPercent"`
	HasTimeInHighPercent     *bool `json:"hasTimeInHighPercent" bson:"hasTimeInHighPercent"`
	HasTimeInVeryHighPercent *bool `json:"hasTimeInVeryHighPercent" bson:"hasTimeInVeryHighPercent"`
	HasTimeInLowPercent      *bool `json:"hasTimeInLowPercent" bson:"hasTimeInLowPercent"`
	HasTimeInVeryLowPercent  *bool `json:"hasTimeInVeryLowPercent" bson:"hasTimeInVeryLowPercent"`

	// actual values
	AverageGlucose *AverageGlucose `json:"averageGlucose" bson:"averageGlucose"`

	TotalRecords        *int     `json:"totalRecords" bson:"totalRecords"`
	AverageDailyRecords *float64 `json:"averageDailyRecords" bson:"averageDailyRecords"`

	TimeInTargetPercent *float64 `json:"timeInTargetPercent" bson:"timeInTargetPercent"`
	TimeInTargetRecords *int     `json:"timeInTargetRecords" bson:"timeInTargetRecords"`

	TimeInLowPercent *float64 `json:"timeInLowPercent" bson:"timeInLowPercent"`
	TimeInLowRecords *int     `json:"timeInLowRecords" bson:"timeInLowRecords"`

	TimeInVeryLowPercent *float64 `json:"timeInVeryLowPercent" bson:"timeInVeryLowPercent"`
	TimeInVeryLowRecords *int     `json:"timeInVeryLowRecords" bson:"timeInVeryLowRecords"`

	TimeInHighPercent *float64 `json:"timeInHighPercent" bson:"timeInHighPercent"`
	TimeInHighRecords *int     `json:"timeInHighRecords" bson:"timeInHighRecords"`

	TimeInVeryHighPercent *float64 `json:"timeInVeryHighPercent" bson:"timeInVeryHighPercent"`
	TimeInVeryHighRecords *int     `json:"timeInVeryHighRecords" bson:"timeInVeryHighRecords"`
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
	HasLastUploadDate *bool      `json:"hasLastUploadDate" bson:"hasLastUploadDate"`
	LastUploadDate    *time.Time `json:"lastUploadDate" bson:"lastUploadDate"`
	LastUpdatedDate   *time.Time `json:"lastUpdatedDate" bson:"lastUpdatedDate"`
	FirstData         *time.Time `json:"firstData" bson:"firstData"`
	LastData          *time.Time `json:"lastData" bson:"lastData"`
	OutdatedSince     *time.Time `json:"outdatedSince" bson:"outdatedSince"`
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
