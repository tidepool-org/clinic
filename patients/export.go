package patients

import (
	"time"
)

type ExportParams struct {
	Period              string
	ExporterClinicianID string
	WorkspaceID         string
	ReportDate          time.Time
}

// ExportedPatient is an intermediate representation of the fields needed by a
// patient export list. It has some looked up fields where simpler / more
// convenient to do it in mongo and other that can be calculated from its
// fields where simpler to do in application code.
type ExportedPatient struct {
	FullName    *string      `bson:"fullName,omitempty"`
	UserId      *string      `bson:"userId,omitempty"`
	MRN         *string      `bson:"mrn,omitempty"`
	BirthDate   *string      `bson:"birthDate,omitempty"`
	Email       *string      `bson:"email,omitempty"`
	Permissions *Permissions `bson:"permissions,omitempty"`
	CreatedTime *time.Time   `bson:"createdTime,omitempty"`
	InvitedBy   *string      `bson:"invitedBy,omitempty"`

	ClinicSiteNames []string        `bson:"clinicSiteNames,omitempty"`
	TagIds          *[]string       `bson:"tagIds,omitempty"`
	GlycemicRanges  *GlycemicRanges `bson:"glycemicRanges,omitempty"`
	DiagnosisType   *string         `bson:"diagnosisType,omitempty"`

	DexcomDataSource *DataSource `bson:"dexcomDataSource,omitempty"`
	AbbottDataSource *DataSource `bson:"abbottDataSource,omitempty"`
	TwiistDataSource *DataSource `bson:"twiistDataSource,omitempty"`

	CgmLastDataDate      *time.Time `bson:"cgmLastData,omitempty"`
	CgmActiveWearTime    *float64   `bson:"cgmActiveWearTime,omitempty"`
	CgmDaysWithData      *int       `bson:"cgmDaysWithData,omitempty"`
	CgmHoursWithData     *int       `bson:"cgmHoursWithData,omitempty"`
	CgmAverageGlucose    *float64   `bson:"cgmAverageGlucose,omitempty"`
	CgmGmi               *float64   `bson:"cgmGmi,omitempty"`
	CgmStdDev            *float64   `bson:"cgmStdDev,omitempty"`
	CgmCV                *float64   `bson:"cgmCV,omitempty"`
	CgmTimeInLevel2Hypo  *float64   `bson:"cgmTimeInLevel2Hypo,omitempty"`
	CgmTimeInLevel1Hypo  *float64   `bson:"cgmTimeInLevel1Hypo,omitempty"`
	CgmTimeInTarget      *float64   `bson:"cgmTimeInTarget,omitempty"`
	CgmTimeInLevel2Hyper *float64   `bson:"cgmTimeInLevel2Hyper,omitempty"`
	CgmTimeInLevel1Hyper *float64   `bson:"cgmTimeInLevel1Hyper,omitempty"`

	BgmLastDataDate   *time.Time `bson:"bgmLastData,omitempty"`
	BgmAverageGlucose *float64   `bson:"bgmAverageGlucose,omitempty"`
	BgmReadingsPerDay *float64   `bson:"bgmReadingsPerDay,omitempty"`
	BgmTotalReadings  *int       `bson:"bgmTotalReadings,omitempty"`
	BgmLowEvents      *int       `bson:"bgmLowEvents,omitempty"`
	BgmHighEvents     *int       `bson:"bgmHighEvents,omitempty"`
}
