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
	List(ctx context.Context, filter *Filter, pagination store.Pagination, sort *store.Sort) (*ListResult, error)
	Create(ctx context.Context, patient Patient) (*Patient, error)
	Update(ctx context.Context, update PatientUpdate) (*Patient, error)
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
	ClinicId                   *string
	UserId                     *string
	Search                     *string
	LastUploadDateFrom         *time.Time
	LastUploadDateTo           *time.Time
	PercentTimeInVeryLowCmp    *string
	PercentTimeInVeryLowValue  float64
	PercentTimeInLowCmp        *string
	PercentTimeInLowValue      float64
	PercentTimeInTargetCmp     *string
	PercentTimeInTargetValue   float64
	PercentTimeInHighCmp       *string
	PercentTimeInHighValue     float64
	PercentTimeInVeryHighCmp   *string
	PercentTimeInVeryHighValue float64
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

type Summary struct {
	AverageGlucose             *AvgGlucose `bson:"averageGlucose,omitempty"`
	FirstData                  *time.Time  `bson:"firstData,omitempty"`
	GlucoseManagementIndicator *float64    `bson:"glucoseManagementIndicator,omitempty"`
	HighGlucoseThreshold       *float64    `bson:"highGlucoseThreshold,omitempty"`
	LastData                   *time.Time  `bson:"lastData,omitempty"`
	LastUpdatedDate            *time.Time  `bson:"lastUpdatedDate,omitempty"`
	LastUploadDate             *time.Time  `bson:"lastUploadDate,omitempty"`
	LowGlucoseThreshold        *float64    `bson:"lowGlucoseThreshold,omitempty"`
	OutdatedSince              *time.Time  `bson:"outdatedSince,omitempty"`
	PercentTimeCGMUse          *float64    `bson:"percentTimeCGMUse,omitempty"`
	PercentTimeInVeryLow       *float64    `bson:"percentTimeInVeryLow,omitempty"`
	PercentTimeInLow           *float64    `bson:"percentTimeInLow,omitempty"`
	PercentTimeInTarget        *float64    `bson:"percentTimeInTarget,omitempty"`
	PercentTimeInHigh          *float64    `bson:"percentTimeInHigh,omitempty"`
	PercentTimeInVeryHigh      *float64    `bson:"percentTimeInVeryHigh,omitempty"`
}

type AvgGlucose struct {
	Units string  `bson:"units"`
	Value float64 `bson:"value"`
}
