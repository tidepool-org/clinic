package summary

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

const (
	SummaryTypeCGM = "cgm"
	SummaryTypeBGM = "bgm"
)

var (
	ErrNotFound = fmt.Errorf("patient %w", errors.NotFound)
)

type Service[T Period] interface {
	Get(ctx context.Context, userId string) (*Summary[T], error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination, sort []*store.Sort) (*ListResult[T], error)
	Remove(ctx context.Context, userId string) error
	CreateOrUpdate(ctx context.Context, summary *Summary[T]) error
}

type ListResult[T Period] struct {
	Patients   []*Summary[T] `bson:"data"`
	TotalCount int           `bson:"count"`
}

type AverageGlucose struct {
	Units string  `bson:"units"`
	Value float64 `bson:"value"`
}

type CGMPeriod struct {
	HasAverageGlucose             bool `json:"hasAverageGlucose" bson:"hasAverageGlucose"`
	HasGlucoseManagementIndicator bool `json:"hasGlucoseManagementIndicator" bson:"hasGlucoseManagementIndicator"`
	HasTimeCGMUsePercent          bool `json:"hasTimeCGMUsePercent" bson:"hasTimeCGMUsePercent"`
	HasTimeInTargetPercent        bool `json:"hasTimeInTargetPercent" bson:"hasTimeInTargetPercent"`
	HasTimeInHighPercent          bool `json:"hasTimeInHighPercent" bson:"hasTimeInHighPercent"`
	HasTimeInVeryHighPercent      bool `json:"hasTimeInVeryHighPercent" bson:"hasTimeInVeryHighPercent"`
	HasTimeInLowPercent           bool `json:"hasTimeInLowPercent" bson:"hasTimeInLowPercent"`
	HasTimeInVeryLowPercent       bool `json:"hasTimeInVeryLowPercent" bson:"hasTimeInVeryLowPercent"`

	// actual values
	TimeCGMUsePercent *float64 `json:"timeCGMUsePercent" bson:"timeCGMUsePercent"`
	TimeCGMUseMinutes int      `json:"timeCGMUseMinutes" bson:"timeCGMUseMinutes"`
	TimeCGMUseRecords int      `json:"timeCGMUseRecords" bson:"timeCGMUseRecords"`

	AverageGlucose             *AverageGlucose `json:"averageGlucose" bson:"averageGlucose"`
	GlucoseManagementIndicator *float64        `json:"glucoseManagementIndicator" bson:"glucoseManagementIndicator"`
	TimeInTargetPercent        *float64        `json:"timeInTargetPercent" bson:"timeInTargetPercent"`
	TimeInTargetMinutes        int             `json:"timeInTargetMinutes" bson:"timeInTargetMinutes"`
	TimeInTargetRecords        int             `json:"timeInTargetRecords" bson:"timeInTargetRecords"`

	TimeInLowPercent *float64 `json:"timeInLowPercent" bson:"timeInLowPercent"`
	TimeInLowMinutes int      `json:"timeInLowMinutes" bson:"timeInLowMinutes"`
	TimeInLowRecords int      `json:"timeInLowRecords" bson:"timeInLowRecords"`

	TimeInVeryLowPercent *float64 `json:"timeInVeryLowPercent" bson:"timeInVeryLowPercent"`
	TimeInVeryLowMinutes int      `json:"timeInVeryLowMinutes" bson:"timeInVeryLowMinutes"`
	TimeInVeryLowRecords int      `json:"timeInVeryLowRecords" bson:"timeInVeryLowRecords"`

	TimeInHighPercent *float64 `json:"timeInHighPercent" bson:"timeInHighPercent"`
	TimeInHighMinutes int      `json:"timeInHighMinutes" bson:"timeInHighMinutes"`
	TimeInHighRecords int      `json:"timeInHighRecords" bson:"timeInHighRecords"`

	TimeInVeryHighPercent *float64 `json:"timeInVeryHighPercent" bson:"timeInVeryHighPercent"`
	TimeInVeryHighMinutes int      `json:"timeInVeryHighMinutes" bson:"timeInVeryHighMinutes"`
	TimeInVeryHighRecords int      `json:"timeInVeryHighRecords" bson:"timeInVeryHighRecords"`
}

type BGMPeriod struct {
	HasAverageGlucose        bool `json:"hasAverageGlucose" bson:"hasAverageGlucose"`
	HasTimeInTargetPercent   bool `json:"hasTimeInTargetPercent" bson:"hasTimeInTargetPercent"`
	HasTimeInHighPercent     bool `json:"hasTimeInHighPercent" bson:"hasTimeInHighPercent"`
	HasTimeInVeryHighPercent bool `json:"hasTimeInVeryHighPercent" bson:"hasTimeInVeryHighPercent"`
	HasTimeInLowPercent      bool `json:"hasTimeInLowPercent" bson:"hasTimeInLowPercent"`
	HasTimeInVeryLowPercent  bool `json:"hasTimeInVeryLowPercent" bson:"hasTimeInVeryLowPercent"`

	// actual values
	AverageGlucose AverageGlucose `json:"averageGlucose" bson:"averageGlucose"`
	TotalRecords   int            `json:"totalRecords" bson:"totalRecords"`

	TimeInTargetPercent float64 `json:"timeInTargetPercent" bson:"timeInTargetPercent"`
	TimeInTargetRecords int     `json:"timeInTargetRecords" bson:"timeInTargetRecords"`

	TimeInLowPercent float64 `json:"timeInLowPercent" bson:"timeInLowPercent"`
	TimeInLowRecords int     `json:"timeInLowRecords" bson:"timeInLowRecords"`

	TimeInVeryLowPercent float64 `json:"timeInVeryLowPercent" bson:"timeInVeryLowPercent"`
	TimeInVeryLowRecords int     `json:"timeInVeryLowRecords" bson:"timeInVeryLowRecords"`

	TimeInHighPercent float64 `json:"timeInHighPercent" bson:"timeInHighPercent"`
	TimeInHighRecords int     `json:"timeInHighRecords" bson:"timeInHighRecords"`

	TimeInVeryHighPercent float64 `json:"timeInVeryHighPercent" bson:"timeInVeryHighPercent"`
	TimeInVeryHighRecords int     `json:"timeInVeryHighRecords" bson:"timeInVeryHighRecords"`
}

func (BGMPeriod) GetType() string {
	return SummaryTypeBGM
}

func (BGMPeriod) Populate(interface{}) {
}

func (s BGMPeriod) Export(dest api.PatientSummary) {
}

func (CGMPeriod) GetType() string {
	return SummaryTypeCGM
}

func (s CGMPeriod) Populate(statsInt interface{}) {
	stats := (statsInt).(api.PatientCGMPeriod)

	var averageGlucose *AverageGlucose

	if stats.AverageGlucose != nil {
		averageGlucose = &AverageGlucose{
			Units: string(stats.AverageGlucose.Units),
			Value: float64(stats.AverageGlucose.Value),
		}
	}

	s.TimeCGMUsePercent = stats.TimeCGMUsePercent
	s.HasTimeCGMUsePercent = *stats.HasTimeCGMUsePercent
	s.TimeCGMUseMinutes = *stats.TimeCGMUseMinutes
	s.TimeCGMUseRecords = *stats.TimeCGMUseRecords

	s.TimeInVeryLowPercent = stats.TimeInVeryLowPercent
	s.HasTimeInVeryLowPercent = *stats.HasTimeInVeryLowPercent
	s.TimeInVeryLowMinutes = *stats.TimeInVeryLowMinutes
	s.TimeInVeryLowRecords = *stats.TimeInVeryLowRecords

	s.TimeInLowPercent = stats.TimeInLowPercent
	s.HasTimeInLowPercent = *stats.HasTimeInLowPercent
	s.TimeInLowMinutes = *stats.TimeInLowMinutes
	s.TimeInLowRecords = *stats.TimeInLowRecords

	s.TimeInTargetPercent = stats.TimeInTargetPercent
	s.HasTimeInTargetPercent = *stats.HasTimeInTargetPercent
	s.TimeInTargetMinutes = *stats.TimeInTargetMinutes
	s.TimeInTargetRecords = *stats.TimeInTargetRecords

	s.TimeInHighPercent = stats.TimeInHighPercent
	s.HasTimeInHighPercent = *stats.HasTimeInHighPercent
	s.TimeInHighMinutes = *stats.TimeInHighMinutes
	s.TimeInHighRecords = *stats.TimeInHighRecords

	s.TimeInVeryHighPercent = stats.TimeInVeryHighPercent
	s.HasTimeInVeryHighPercent = *stats.HasTimeInVeryHighPercent
	s.TimeInVeryHighMinutes = *stats.TimeInVeryHighMinutes
	s.TimeInVeryHighRecords = *stats.TimeInVeryHighRecords

	s.GlucoseManagementIndicator = stats.GlucoseManagementIndicator
	s.HasGlucoseManagementIndicator = *stats.HasGlucoseManagementIndicator
	s.AverageGlucose = averageGlucose
	s.HasAverageGlucose = *stats.HasAverageGlucose
}

func (s CGMPeriod) Export(dest api.PatientSummary) {
	var avgGlucose api.AverageGlucose
	if s.AverageGlucose != nil {
		avgGlucose.Value = float32(s.AverageGlucose.Value)
		avgGlucose.Units = api.AverageGlucoseUnits(s.AverageGlucose.Units)
	}

	destStats := api.PatientCGMPeriod{
		AverageGlucose:                &avgGlucose,
		HasAverageGlucose:             &s.HasAverageGlucose,
		GlucoseManagementIndicator:    s.GlucoseManagementIndicator,
		HasGlucoseManagementIndicator: &s.HasGlucoseManagementIndicator,
		HasTimeCGMUsePercent:          &s.HasTimeCGMUsePercent,
		HasTimeInHighPercent:          &s.HasTimeInHighPercent,
		HasTimeInLowPercent:           &s.HasTimeInLowPercent,
		HasTimeInTargetPercent:        &s.HasTimeInTargetPercent,
		HasTimeInVeryHighPercent:      &s.HasTimeInVeryHighPercent,
		HasTimeInVeryLowPercent:       &s.HasTimeInVeryLowPercent,
		TimeCGMUseMinutes:             &s.TimeCGMUseMinutes,
		TimeCGMUsePercent:             s.TimeCGMUsePercent,
		TimeCGMUseRecords:             &s.TimeCGMUseRecords,
		TimeInHighMinutes:             &s.TimeInHighMinutes,
		TimeInHighPercent:             s.TimeInHighPercent,
		TimeInHighRecords:             &s.TimeInHighRecords,
		TimeInLowMinutes:              &s.TimeInLowMinutes,
		TimeInLowPercent:              s.TimeInLowPercent,
		TimeInLowRecords:              &s.TimeInLowRecords,
		TimeInTargetMinutes:           &s.TimeInTargetMinutes,
		TimeInTargetPercent:           s.TimeInTargetPercent,
		TimeInTargetRecords:           &s.TimeInTargetRecords,
		TimeInVeryHighMinutes:         &s.TimeInVeryHighMinutes,
		TimeInVeryHighPercent:         s.TimeInVeryHighPercent,
		TimeInVeryHighRecords:         &s.TimeInVeryHighRecords,
		TimeInVeryLowMinutes:          &s.TimeInVeryLowMinutes,
		TimeInVeryLowPercent:          s.TimeInVeryLowPercent,
		TimeInVeryLowRecords:          &s.TimeInVeryLowRecords,
	}

	var destStatsInt interface{}
	destStatsInt = destStats
	dest.Stats = &destStatsInt
}

func GetTypeString[T Period]() string {
	s := new(Summary[T])
	return s.Stats.GetType()
}

type Config struct {
	SchemaVersion int `json:"schemaVersion" bson:"schemaVersion"`

	// these are just constants right now.
	HighGlucoseThreshold     float64 `json:"highGlucoseThreshold" bson:"highGlucoseThreshold"`
	VeryHighGlucoseThreshold float64 `json:"veryHighGlucoseThreshold" bson:"veryHighGlucoseThreshold"`
	LowGlucoseThreshold      float64 `json:"lowGlucoseThreshold" bson:"lowGlucoseThreshold"`
	VeryLowGlucoseThreshold  float64 `json:"VeryLowGlucoseThreshold" bson:"VeryLowGlucoseThreshold"`
}

type Dates struct {
	TotalHours int `json:"totalHours" bson:"totalHours"`

	// date tracking
	HasLastUploadDate bool      `json:"hasLastUploadDate" bson:"hasLastUploadDate"`
	LastUploadDate    time.Time `json:"lastUploadDate" bson:"lastUploadDate"`
	LastUpdatedDate   time.Time `json:"lastUpdatedDate" bson:"lastUpdatedDate"`
	FirstData         time.Time `json:"firstData" bson:"firstData"`
	LastData          time.Time `json:"lastData" bson:"lastData"`
	OutdatedSince     time.Time `json:"outdatedSince" bson:"outdatedSince"`
}

type Period interface {
	CGMPeriod | BGMPeriod

	GetType() string
	Populate(interface{})
	Export(summary api.PatientSummary)
}

type EmbeddedPatient struct {
	// perhaps move clinicid into a list under summary if better on indexing?
	ClinicId primitive.ObjectID `json:"clinicId" bson:"clinicId"`

	BirthDate *string `bson:"birthDate"`
	Email     *string `bson:"email"`
	FullName  *string `bson:"fullName"`
	Mrn       *string `bson:"mrn"`
}

type Summary[T Period] struct {
	ID     primitive.ObjectID `json:"-" bson:"_id,omitempty"`
	UserID api.TidepoolUserId `json:"userId" bson:"userId"`

	Type   string `json:"type" bson:"type"`
	Period string `json:"period" bson:"period"`

	Patients []EmbeddedPatient `json:"patients" bson:"patients"`
	Config   Config            `json:"config" bson:"config"`
	Dates    Dates             `json:"dates" bson:"dates"`
	Stats    T                 `json:"stats" bson:"stats"`
}

type Filter struct {
	ClinicId           *string
	UserId             *string
	Type               *string
	Search             *string
	LastUploadDateFrom *time.Time
	LastUploadDateTo   *time.Time

	TimeCGMUsePercentCmp       *string
	TimeCGMUsePercentValue     float64
	TimeInVeryLowPercentCmp    *string
	TimeInVeryLowPercentValue  float64
	TimeInLowPercentCmp        *string
	TimeInLowPercentValue      float64
	TimeInTargetPercentCmp     *string
	TimeInTargetPercentValue   float64
	TimeInHighPercentCmp       *string
	TimeInHighPercentValue     float64
	TimeInVeryHighPercentCmp   *string
	TimeInVeryHighPercentValue float64
}
