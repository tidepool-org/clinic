package patients

import (
	"context"
	"fmt"
	"reflect"
	"strings"
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
	UpdatePatientSummary(ctx context.Context, patientId string, summary *Summary) error
	UpdateLastUploadReminderTime(ctx context.Context, update *UploadReminderUpdate) (*Patient, error)
	UpdateLastRequestedDexcomConnectTime(ctx context.Context, update *LastRequestedDexcomConnectUpdate) (*Patient, error)
	AssignPatientTagToClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error
	DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error
	UpdatePatientDataSources(ctx context.Context, userId string, dataSources *DataSources) error
	TideReport(ctx context.Context, clinicId string, pagination store.Pagination, params TideReportParams) (*Tide, error)
	ListPatientsForUserId(ctx context.Context, userId string) ([]*Patient, error)
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
	ClinicId *string
	UserId   *string
	Search   *string
	Tags     *[]string
	Period   *string

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

type Summary struct {
	CGM  *PatientCGMStats   `json:"cgmStats" bson:"cgmStats"`
	BGM  *PatientBGMStats   `json:"bgmStats" bson:"bgmStats"`
	Risk PatientRiskPeriods `json:"risk" bson:"risk"`
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
	Category              *string
	Period                *string
	Tags                  *[]string
	CgmLastUploadDateFrom *time.Time
	CgmLastUploadDateTo   *time.Time
}

type PatientRiskCategories [][]byte
type PatientRiskPeriods map[string]*PatientRiskCategories

func periodByJsonTag(s PatientCGMPeriod) map[string]*float64 {
	valuesByTag := make(map[string]*float64)

	typeOf := reflect.TypeOf(s)
	valueOf := reflect.ValueOf(s)

	for i := 0; i < valueOf.NumField(); i++ {
		f := typeOf.Field(i)
		key := strings.Split(f.Tag.Get("json"), ",")[0]
		if key == "" || key == "-" {
			continue
		}
		vInt := valueOf.Field(i).Interface()
		switch v := vInt.(type) {
		case *float64:
			valuesByTag[key] = v
		case *int:
			if v != nil {
				c := float64(*v)
				valuesByTag[key] = &c
			} else {
				valuesByTag[key] = nil
			}
		}
	}

	return valuesByTag
}

func (s *Summary) TideCategorize(tideConfig []*TideFilters) {
	ops := map[string]func(float64, float64) bool{
		"<":  func(x, y float64) bool { return x < y },
		">":  func(x, y float64) bool { return x > y },
		">=": func(x, y float64) bool { return x >= y },
		"<=": func(x, y float64) bool { return x <= y },
		"==": func(x, y float64) bool { return x == y },
		"!=": func(x, y float64) bool { return x != y },
	}

	s.Risk = make(PatientRiskPeriods)
	var empty struct{}

	for periodKey, period := range *s.CGM.Periods {
		periodByTag := periodByJsonTag(period)
		riskCategoriesMap := make(map[string]struct{})

		for _, report := range tideConfig {
			for _, category := range *report {
				if periodByTag[*category.Field] != nil {
					if ops[*category.Comparison](*periodByTag[*category.Field], *category.Value) {
						riskCategoriesMap[string(*category.Id)] = empty

						// NOTE we only allow the first match per report to be added
						break
					}
				}
			}
		}

		riskCategories := make(PatientRiskCategories, 0, 2)
		for k, _ := range riskCategoriesMap {
			riskCategories = append(riskCategories, []byte(k))
		}
		s.Risk[periodKey] = &riskCategories
	}
}
