package export

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
)

const (
	timeFormat = "2006-01-02 03:04 PM"
)

type exporter struct {
	patientSvc patients.Service

	clinic                 *clinics.Clinic
	tagNamesById           map[string]string
	clinicianNamesByUserId map[string]string
	exportingClinician     *clinicians.Clinician
	params                 patients.ExportParams
	days                   int
}

func (e *exporter) ToCSVRow(p *patients.ExportedPatient) []string {
	return []string{
		pstr(p.FullName),
		pstr(p.UserId),
		pstr(p.MRN),
		fmtBirthdate(p.BirthDate),
		pstr(p.Email),
		fmtBool(p.Permissions.IsClaimed(), "Claimed", "Unclaimed"),
		ptime(p.CreatedTime, "2006-01-02"),
		e.clinicianNamesByUserId[pstr(p.InvitedBy)],
		strings.Join(p.ClinicSiteNames, ","),
		fmtPatientTags(p.TagIds, e.tagNamesById),
		fmtGlycemicRanges(p.GlycemicRanges),
		pstrd(p.DiagnosisType, "Other"),
		fmtDataSourceStatus(p.DexcomDataSource, e.params.ReportDate),
		fmtDataSourceLastDataDate(p.DexcomDataSource),
		fmtDataSourceStatus(p.AbbottDataSource, e.params.ReportDate),
		fmtDataSourceLastDataDate(p.AbbottDataSource),
		fmtDataSourceStatus(p.TwiistDataSource, e.params.ReportDate),
		fmtDataSourceLastDataDate(p.TwiistDataSource),
		ptime(p.CgmLastDataDate, "2006-01-02"),
		ppct(p.CgmActiveWearTime, 0),
		pint(p.CgmDaysWithData),
		pint(p.CgmHoursWithData),
		ptomgdl(p.CgmAverageGlucose),
		pfloat(p.CgmGmi, 2),
		fmtPreferredUnits(p.CgmStdDev, e.clinic.PreferredBgUnits, 1),
		ppct(p.CgmCV, 0),
		ppct(p.CgmTimeInLevel2Hypo, 0),
		ppct(p.CgmTimeInLevel1Hypo, 0),
		ppct(p.CgmTimeInTarget, 0),
		ppct(p.CgmTimeInLevel1Hyper, 0),
		ppct(p.CgmTimeInLevel2Hyper, 0),
		ptime(p.BgmLastDataDate, "2006-01-02"),
		ptomgdl(p.BgmAverageGlucose),
		pfloat(p.BgmReadingsPerDay, 0),
		pint(p.BgmTotalReadings),
		pint(p.BgmLowEvents),
		pint(p.BgmHighEvents),
	}
}

func NewPatientExport(ctx context.Context, clinicSvc clinics.Service, clinicianSvc clinicians.Service, patientSvc patients.Service, params patients.ExportParams) (*exporter, error) {
	clinic, err := clinicSvc.Get(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}

	pagination := store.DefaultPagination().WithLimit(1000)
	filter := &clinicians.Filter{
		ClinicId: &params.WorkspaceID,
	}
	cs, err := clinicianSvc.List(ctx, filter, pagination)
	if err != nil {
		return nil, err
	}
	return NewPatientExportClinic(clinic, cs, patientSvc, params)
}

func NewPatientExportClinic(clinic *clinics.Clinic, cs []*clinicians.Clinician, patientSvc patients.Service, params patients.ExportParams) (*exporter, error) {
	days, err := periodToDays(params.Period)
	if err != nil {
		return nil, err
	}

	tagNamesById := map[string]string{}
	for _, patientTag := range clinic.PatientTags {
		tagNamesById[patientTag.Id.Hex()] = patientTag.Name
	}

	clinicianNamesByUserId := map[string]string{}
	var exportingClinician *clinicians.Clinician
	for _, clinician := range cs {
		if clinician.UserId != nil && clinician.Name != nil {
			clinicianNamesByUserId[*clinician.UserId] = pstr(clinician.Name)
		}
		if clinician.UserId != nil && pstr(clinician.UserId) == params.ExporterClinicianID {
			exportingClinician = clinician
		}
	}
	if exportingClinician == nil {
		return nil, fmt.Errorf(`no clinician "%v" found.`, params.ExporterClinicianID)
	}
	return &exporter{
		patientSvc:             patientSvc,
		clinic:                 clinic,
		exportingClinician:     exportingClinician,
		tagNamesById:           tagNamesById,
		clinicianNamesByUserId: clinicianNamesByUserId,
		params:                 params,
		days:                   days,
	}, nil
}

func (e *exporter) Write(ctx context.Context, w io.Writer) error {
	ps, err := e.patientSvc.ListExportedPatients(ctx, e.params)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(w)
	metadataTitles := []string{
		"Report Date Time",
		"Exported By",
		"Exported By",
		"Clinic Name",
		"Workspace ID",
		"Days of CGM Data Summarized",
		"Total Patients",
	}
	metadata := []string{
		fmtClinicTime(e.params.ReportDate, e.clinic),
		pstr(e.exportingClinician.Name),
		pstr(e.exportingClinician.Email),
		pstr(e.clinic.Name),
		e.clinic.Id.Hex(),
		fmtDays(e.days),
		fmt.Sprintf("%v", len(ps)),
	}

	if err := writer.Write(metadataTitles); err != nil {
		return err
	}
	if err := writer.Write(metadata); err != nil {
		return err
	}

	header := []string{
		"Patient Name",
		"Patient User ID",
		"MRN",
		"Date of Birth",
		"Patient Email",
		"Custodial Status",
		"Account Created Date",
		"Clinician Who Created Account",
		"Clinic Sites",
		"Patient Tags",
		"Assigned Glycemic Range",
		"Diabetes Type",
		"Dexcom Status",
		"Dexcom Last Modified Date",
		"Abbott Status",
		"Abbott Last Modified Date",
		"Twiist Status",
		"Twiist Last Modified Date",
		"CGM Last Data Date",
		"CGM Active Wear Time (%)",
		"CGM Days with Data",
		"CGM Hours with Data",
		"CGM Average Glucose (mg/dL)",
		"CGM GMI (%)",
		fmt.Sprintf("CGM Standard Deviation (%s)", e.clinic.PreferredBgUnits),
		"CGM Coefficient of Variation (%)",
		"CGM Time in Level 2 Hypoglycemia (%)",
		"CGM Time in Level 1 Hypoglycemia (%)",
		"CGM Time in Target (%)",
		"CGM Time in Level 1 Hyperglycemia (%)",
		"CGM Time in Level 2 Hyperglycemia (%)",
		"BGM Last Data Date",
		"BGM Average Glucose",
		"BGM Readings/Day",
		"BGM Total Readings",
		"BGM Low Events",
		"BGM High Events",
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, patient := range ps {
		row := e.ToCSVRow(&patient)
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	return nil
}

func fmtDays(days int) string {
	if days == 1 {
		return "24 hours"
	}
	return fmt.Sprintf("%d days", days)
}

func fmtDataSourceLastDataDate(ds *patients.DataSource) string {
	if ds == nil || ds.LatestDataTime == nil {
		return ""
	}
	return ds.LatestDataTime.Format(time.DateOnly)
}

func fmtDataSourceStatus(ds *patients.DataSource, now time.Time) string {
	if ds == nil || ds.State == "" {
		return "NA"
	}
	inactiveCutoff := now.Add(-time.Hour * 24 * 2)
	expiredCutoff := now
	if (ds.State == patients.DataSourceStatePending || ds.State == patients.DataSourceStatePendingReconnect) && ds.ExpirationTime != nil && ds.ExpirationTime.Before(expiredCutoff) {
		return "expired"
	}
	if ds.State == "connected" && ds.LatestDataTime != nil && ds.LatestDataTime.Before(inactiveCutoff) {
		return "inactive"
	}
	return ds.State
}

func fmtClinicTime(t time.Time, clinic *clinics.Clinic) string {
	if clinic.Timezone == nil || *clinic.Timezone == "" {
		return t.Format(timeFormat)
	}
	loc, err := time.LoadLocation(*clinic.Timezone)
	if err != nil {
		return t.Format(timeFormat)
	}
	return t.In(loc).Format(timeFormat)
}

func fmtClinicDate(t time.Time, clinic *clinics.Clinic) string {
	if clinic.Timezone == nil || *clinic.Timezone == "" {
		return t.Format(time.DateOnly)
	}
	loc, err := time.LoadLocation(*clinic.Timezone)
	if err != nil {
		return t.Format(time.DateOnly)
	}
	return t.In(loc).Format(time.DateOnly)
}

func fmtBool(b bool, valIfTrue string, valIfFalse string) string {
	if b {
		return valIfTrue
	}
	return valIfFalse
}

func fmtGlycemicRanges(gr *patients.GlycemicRanges) string {
	if gr == nil {
		return ""
	}
	if gr.Type == patients.GlycemicRangeTypePreset {
		return fmt.Sprintf("%v", gr.Preset)
	}
	return gr.Custom.Name
}

func fmtPatientTags(tagIds *[]string, tagNamesById map[string]string) string {
	if tagIds == nil || len(*tagIds) == 0 {
		return ""
	}
	tagNames := make([]string, len(*tagIds))
	for i, tagId := range *tagIds {
		if name, ok := tagNamesById[tagId]; ok {
			tagNames[i] = name
		}
	}
	return strings.Join(tagNames, ",")
}

func fmtBirthdate(birthDate *string) string {
	if birthDate == nil || *birthDate == "" {
		return ""
	}
	t, err := time.Parse(time.DateOnly, *birthDate)
	if err != nil || t.IsZero() {
		return ""
	}
	return t.Format(time.DateOnly)
}

func periodToDays(period string) (days int, err error) {
	days, ok := map[string]int{
		"1d":  1,
		"7d":  7,
		"14d": 14,
		"30d": 30,
	}[period]
	if !ok {
		return 0, fmt.Errorf(`no days for given period, "%v"`, period)
	}
	return days, nil
}

func fmtFloat(f float64, precision int) string {
	shift := math.Pow(10, float64(precision))
	return fmt.Sprintf("%v", math.RoundToEven(f*shift*100)/shift)
}

func fmtPreferredUnits(valMmolL *float64, preferredBgUnits string, precision int) string {
	if valMmolL == nil {
		return ""
	}
	if strings.ToLower(preferredBgUnits) == "mg/dl" {
		return fmtFloat(toMgDl(*valMmolL), precision)
	}
	return fmtFloat(*valMmolL, precision)
}
