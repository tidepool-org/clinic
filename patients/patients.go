package patients

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

var (
	ErrNotFound           = fmt.Errorf("patient %w", errors.NotFound)
	ErrPermissionNotFound = fmt.Errorf("permission %w", errors.NotFound)
	ErrDuplicatePatient   = fmt.Errorf("%w: patient is already a member of the clinic", errors.Duplicate)
	ErrDuplicateEmail     = fmt.Errorf("%w: email address is already taken", errors.Duplicate)

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
}

type Patient struct {
	Id                     *primitive.ObjectID `bson:"_id,omitempty"`
	ClinicId               *primitive.ObjectID `bson:"clinicId,omitempty"`
	UserId                 *string             `bson:"userId,omitempty"`
	BirthDate              *string             `bson:"birthDate"`
	Email                  *string             `bson:"email"`
	FullName               *string             `bson:"fullName"`
	Mrn                    *string             `bson:"mrn"`
	TargetDevices          *[]string           `bson:"targetDevices"`
	Permissions            *Permissions        `bson:"permissions,omitempty"`
	IsMigrated             bool                `bson:"isMigrated,omitempty"`
	LegacyClinicianIds     []string            `bson:"legacyClinicianIds,omitempty"`
	CreatedTime            time.Time           `bson:"createdTime,omitempty"`
	UpdatedTime            time.Time           `bson:"updatedTime,omitempty"`
	InvitedBy              *string             `bson:"invitedBy,omitempty"`
	Summary                *Summary            `bson:"summary,omitempty"`
	LastUploadReminderTime time.Time           `bson:"lastUploadReminderTime,omitempty"`
}

// PatientSummary defines model for PatientSummary.

func (p Patient) IsCustodial() bool {
	return p.Permissions != nil && p.Permissions.Custodian != nil
}

type Filter struct {
	ClinicId           *string
	UserId             *string
	Search             *string
	LastUploadDateFrom *time.Time
	LastUploadDateTo   *time.Time

	TimeCGMUsePercentCmp1d       *string
	TimeCGMUsePercentValue1d     float64
	TimeInVeryLowPercentCmp1d    *string
	TimeInVeryLowPercentValue1d  float64
	TimeInLowPercentCmp1d        *string
	TimeInLowPercentValue1d      float64
	TimeInTargetPercentCmp1d     *string
	TimeInTargetPercentValue1d   float64
	TimeInHighPercentCmp1d       *string
	TimeInHighPercentValue1d     float64
	TimeInVeryHighPercentCmp1d   *string
	TimeInVeryHighPercentValue1d float64

	TimeCGMUsePercentCmp7d       *string
	TimeCGMUsePercentValue7d     float64
	TimeInVeryLowPercentCmp7d    *string
	TimeInVeryLowPercentValue7d  float64
	TimeInLowPercentCmp7d        *string
	TimeInLowPercentValue7d      float64
	TimeInTargetPercentCmp7d     *string
	TimeInTargetPercentValue7d   float64
	TimeInHighPercentCmp7d       *string
	TimeInHighPercentValue7d     float64
	TimeInVeryHighPercentCmp7d   *string
	TimeInVeryHighPercentValue7d float64

	TimeCGMUsePercentCmp14d       *string
	TimeCGMUsePercentValue14d     float64
	TimeInVeryLowPercentCmp14d    *string
	TimeInVeryLowPercentValue14d  float64
	TimeInLowPercentCmp14d        *string
	TimeInLowPercentValue14d      float64
	TimeInTargetPercentCmp14d     *string
	TimeInTargetPercentValue14d   float64
	TimeInHighPercentCmp14d       *string
	TimeInHighPercentValue14d     float64
	TimeInVeryHighPercentCmp14d   *string
	TimeInVeryHighPercentValue14d float64

	TimeCGMUsePercentCmp30d       *string
	TimeCGMUsePercentValue30d     float64
	TimeInVeryLowPercentCmp30d    *string
	TimeInVeryLowPercentValue30d  float64
	TimeInLowPercentCmp30d        *string
	TimeInLowPercentValue30d      float64
	TimeInTargetPercentCmp30d     *string
	TimeInTargetPercentValue30d   float64
	TimeInHighPercentCmp30d       *string
	TimeInHighPercentValue30d     float64
	TimeInVeryHighPercentCmp30d   *string
	TimeInVeryHighPercentValue30d float64
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

type CGMStats struct {
	Date *time.Time `json:"date" bson:"date"`

	TargetMinutes *int `json:"targetMinutes" bson:"targetMinutes"`
	TargetRecords *int `json:"targetRecords" bson:"targetRecords"`

	LowMinutes *int `json:"lowMinutes" bson:"lowMinutes"`
	LowRecords *int `json:"lowRecords" bson:"lowRecords"`

	VeryLowMinutes *int `json:"veryLowMinutes" bson:"veryLowMinutes"`
	VeryLowRecords *int `json:"veryLowRecords" bson:"veryLowRecords"`

	HighMinutes *int `json:"highMinutes" bson:"highMinutes"`
	HighRecords *int `json:"highRecords" bson:"highRecords"`

	VeryHighMinutes *int `json:"veryHighMinutes" bson:"veryHighMinutes"`
	VeryHighRecords *int `json:"veryHighRecords" bson:"veryHighRecords"`

	TotalGlucose *float64 `json:"totalGlucose" bson:"totalGlucose"`
	TotalMinutes *int     `json:"totalMinutes" bson:"totalMinutes"`
	TotalRecords *int     `json:"totalRecords" bson:"totalRecords"`

	LastRecordTime *time.Time `json:"lastRecordTime" bson:"lastRecordTime"`
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

	AverageGlucose             *AverageGlucose `json:"averageGlucose" bson:"avgGlucose"`
	GlucoseManagementIndicator *float64        `json:"glucoseManagementIndicator" bson:"glucoseManagementIndicator"`

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

type CGMSummary struct {
	Periods     map[string]*CGMPeriod `json:"periods" bson:"periods"`
	HourlyStats []*CGMStats           `json:"hourlyStats" bson:"hourlyStats"`
	TotalHours  *int                  `json:"totalHours" bson:"totalHours"`

	// date tracking
	HasLastUploadDate *bool      `json:"hasLastUploadDate" bson:"hasLastUploadDate"`
	LastUploadDate    *time.Time `json:"lastUploadDate" bson:"lastUploadDate"`
	LastUpdatedDate   *time.Time `json:"lastUpdatedDate" bson:"lastUpdatedDate"`
	FirstData         *time.Time `json:"firstData" bson:"firstData"`
	LastData          *time.Time `json:"lastData" bson:"lastData"`
	OutdatedSince     *time.Time `json:"outdatedSince" bson:"outdatedSince"`
}

type BGMStats struct {
	Date time.Time `json:"date" bson:"date"`

	TargetRecords   *int `json:"targetRecords" bson:"targetRecords"`
	LowRecords      *int `json:"lowRecords" bson:"lowRecords"`
	VeryLowRecords  *int `json:"veryLowRecords" bson:"veryLowRecords"`
	HighRecords     *int `json:"highRecords" bson:"highRecords"`
	VeryHighRecords *int `json:"veryHighRecords" bson:"veryHighRecords"`

	TotalGlucose *float64 `json:"totalGlucose" bson:"totalGlucose"`
	TotalRecords *int     `json:"totalRecords" bson:"totalRecords"`

	LastRecordTime *time.Time `json:"lastRecordTime" bson:"lastRecordTime"`
}

type BGMPeriod struct {
	HasAverageGlucose        *bool `json:"hasAverageGlucose" bson:"hasAverageGlucose"`
	HasTimeInTargetPercent   *bool `json:"hasTimeInTargetPercent" bson:"hasTimeInTargetPercent"`
	HasTimeInHighPercent     *bool `json:"hasTimeInHighPercent" bson:"hasTimeInHighPercent"`
	HasTimeInVeryHighPercent *bool `json:"hasTimeInVeryHighPercent" bson:"hasTimeInVeryHighPercent"`
	HasTimeInLowPercent      *bool `json:"hasTimeInLowPercent" bson:"hasTimeInLowPercent"`
	HasTimeInVeryLowPercent  *bool `json:"hasTimeInVeryLowPercent" bson:"hasTimeInVeryLowPercent"`

	// actual values
	AverageGlucose *AverageGlucose `json:"averageGlucose" bson:"avgGlucose"`

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

type BGMSummary struct {
	Periods     map[string]*BGMPeriod `json:"periods" bson:"periods"`
	HourlyStats []*BGMStats           `json:"hourlyStats" bson:"hourlyStats"`
	TotalHours  *int                  `json:"totalHours" bson:"totalHours"`

	// date tracking
	HasLastUploadDate *bool      `json:"hasLastUploadDate" bson:"hasLastUploadDate"`
	LastUploadDate    *time.Time `json:"lastUploadDate" bson:"lastUploadDate"`
	LastUpdatedDate   *time.Time `json:"lastUpdatedDate" bson:"lastUpdatedDate"`
	OutdatedSince     *time.Time `json:"outdatedSince" bson:"outdatedSince"`
	FirstData         *time.Time `json:"firstData" bson:"firstData"`
	LastData          *time.Time `json:"lastData" bson:"lastData"`
}

type Config struct {
	SchemaVersion *int `json:"schemaVersion" bson:"schemaVersion"`

	// these are just constants right now.
	HighGlucoseThreshold     *float64 `json:"highGlucoseThreshold" bson:"highGlucoseThreshold"`
	VeryHighGlucoseThreshold *float64 `json:"veryHighGlucoseThreshold" bson:"veryHighGlucoseThreshold"`
	LowGlucoseThreshold      *float64 `json:"lowGlucoseThreshold" bson:"lowGlucoseThreshold"`
	VeryLowGlucoseThreshold  *float64 `json:"VeryLowGlucoseThreshold" bson:"VeryLowGlucoseThreshold"`
}

type Summary struct {
	CGM CGMSummary `json:"cgmSummary" bson:"cgmSummary"`
	BGM BGMSummary `json:"bgmSummary" bson:"bgmSummary"`

	Config Config `json:"config" bson:"config"`
}

type AverageGlucose struct {
	Units string  `bson:"units"`
	Value float64 `bson:"value"`
}
