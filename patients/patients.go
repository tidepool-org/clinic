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

type Period struct {
	TimeCGMUsePercent    *float64 `bson:"timeCGMUsePercent"`
	HasTimeCGMUsePercent *bool    `bson:"hasTimeCGMUsePercent"`
	TimeCGMUseMinutes    *int     `bson:"timeCGMUseMinutes"`
	TimeCGMUseRecords    *int     `bson:"timeCGMUseRecords"`

	AverageGlucose    *AverageGlucose `bson:"averageGlucose"`
	HasAverageGlucose *bool           `bson:"hasAverageGlucose"`

	GlucoseManagementIndicator    *float64 `bson:"glucoseManagementIndicator"`
	HasGlucoseManagementIndicator *bool    `bson:"hasGlucoseManagementIndicator"`

	TimeInTargetPercent    *float64 `bson:"timeInTargetPercent"`
	HasTimeInTargetPercent *bool    `bson:"hasTimeInTargetPercent"`
	TimeInTargetMinutes    *int     `bson:"timeInTargetMinutes"`
	TimeInTargetRecords    *int     `bson:"timeInTargetRecords"`

	TimeInLowPercent    *float64 `bson:"timeInLowPercent"`
	HasTimeInLowPercent *bool    `bson:"hasTimeInLowPercent"`
	TimeInLowMinutes    *int     `bson:"timeInLowMinutes"`
	TimeInLowRecords    *int     `bson:"timeInLowRecords"`

	TimeInVeryLowPercent    *float64 `bson:"timeInVeryLowPercent"`
	HasTimeInVeryLowPercent *bool    `bson:"hasTimeInVeryLowPercent"`
	TimeInVeryLowMinutes    *int     `bson:"timeInVeryLowMinutes"`
	TimeInVeryLowRecords    *int     `bson:"timeInVeryLowRecords"`

	TimeInHighPercent    *float64 `bson:"timeInHighPercent"`
	HasTimeInHighPercent *bool    `bson:"hasTimeInHighPercent"`
	TimeInHighMinutes    *int     `bson:"timeInHighMinutes"`
	TimeInHighRecords    *int     `bson:"timeInHighRecords"`

	TimeInVeryHighPercent    *float64 `bson:"timeInVeryHighPercent"`
	HasTimeInVeryHighPercent *bool    `bson:"hasTimeInVeryHighPercent"`
	TimeInVeryHighMinutes    *int     `bson:"timeInVeryHighMinutes"`
	TimeInVeryHighRecords    *int     `bson:"timeInVeryHighRecords"`
}

type Summary struct {
	Periods map[string]*Period `bson:"periods"`

	FirstData         *time.Time `bson:"firstData"`
	LastData          *time.Time `bson:"lastData"`
	LastUpdatedDate   *time.Time `bson:"lastUpdatedDate"`
	LastUploadDate    *time.Time `bson:"lastUploadDate"`
	HasLastUploadDate *bool      `bson:"hasLastUploadDate"`
	OutdatedSince     *time.Time `bson:"outdatedSince"`
	TotalHours        *int       `bson:"totalHours"`

	HighGlucoseThreshold     *float64 `bson:"highGlucoseThreshold"`
	VeryHighGlucoseThreshold *float64 `bson:"veryHighGlucoseThreshold"`
	LowGlucoseThreshold      *float64 `bson:"lowGlucoseThreshold"`
	VeryLowGlucoseThreshold  *float64 `bson:"VeryLowGlucoseThreshold"`
}

type AverageGlucose struct {
	Units string  `bson:"units"`
	Value float64 `bson:"value"`
}
