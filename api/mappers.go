package api

import (
	"fmt"
	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/migration"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func NewClinic(c Clinic) *clinics.Clinic {
	var phoneNumbers []clinics.PhoneNumber
	if c.PhoneNumbers != nil {
		for _, n := range *c.PhoneNumbers {
			phoneNumbers = append(phoneNumbers, clinics.PhoneNumber{
				Number: n.Number,
				Type:   n.Type,
			})
		}
	}

	return &clinics.Clinic{
		Name:             &c.Name,
		ClinicType:       clinicTypeToString(c.ClinicType),
		ClinicSize:       clinicSizeToString(c.ClinicSize),
		Address:          c.Address,
		City:             c.City,
		Country:          c.Country,
		PostalCode:       c.PostalCode,
		State:            c.State,
		PhoneNumbers:     &phoneNumbers,
		Website:          c.Website,
		PreferredBgUnits: string(c.PreferredBgUnits),
	}
}

func NewClinicDto(c *clinics.Clinic) Clinic {
	tier := clinics.DefaultTier
	if c.Tier != "" {
		tier = c.Tier
	}

	units := ClinicPreferredBgUnitsMgdL
	if c.PreferredBgUnits != "" {
		units = ClinicPreferredBgUnits(c.PreferredBgUnits)
	}
	dto := Clinic{
		Id:               Id(c.Id.Hex()),
		Name:             pstr(c.Name),
		ShareCode:        pstr(c.CanonicalShareCode),
		CanMigrate:       c.CanMigrate(),
		ClinicType:       stringToClinicType(c.ClinicType),
		ClinicSize:       stringToClinicSize(c.ClinicSize),
		Address:          c.Address,
		City:             c.City,
		PostalCode:       c.PostalCode,
		State:            c.State,
		Country:          c.Country,
		Website:          c.Website,
		CreatedTime:      c.CreatedTime,
		UpdatedTime:      c.UpdatedTime,
		Tier:             tier,
		TierDescription:  clinics.GetTierDescription(tier),
		PreferredBgUnits: units,
	}
	if c.PhoneNumbers != nil {
		var phoneNumbers []PhoneNumber
		for _, n := range *c.PhoneNumbers {
			phoneNumbers = append(phoneNumbers, PhoneNumber{
				Number: n.Number,
				Type:   n.Type,
			})
		}
		dto.PhoneNumbers = &phoneNumbers
	}

	return dto
}

func NewClinicsDto(clinics []*clinics.Clinic) []Clinic {
	dtos := make([]Clinic, 0)
	for _, clinic := range clinics {
		dtos = append(dtos, NewClinicDto(clinic))
	}
	return dtos
}

func NewClinicianDto(clinician *clinicians.Clinician) Clinician {
	dto := Clinician{
		Id:          strpuseridp(clinician.UserId),
		InviteId:    clinician.InviteId,
		Name:        clinician.Name,
		Email:       pstr(clinician.Email),
		Roles:       ClinicianRoles(clinician.Roles),
		CreatedTime: clinician.CreatedTime,
		UpdatedTime: clinician.UpdatedTime,
	}
	return dto
}

func NewCliniciansDto(clinicians []*clinicians.Clinician) []Clinician {
	dtos := make([]Clinician, 0)
	for _, c := range clinicians {
		if c != nil {
			dtos = append(dtos, NewClinicianDto(c))
		}
	}
	return dtos
}

func NewClinician(clinician Clinician) *clinicians.Clinician {
	return &clinicians.Clinician{
		Name:     clinician.Name,
		UserId:   useridpstrp(clinician.Id),
		InviteId: clinician.InviteId,
		Roles:    clinician.Roles,
		Email:    strp(strings.ToLower(clinician.Email)),
	}
}

func NewClinicianUpdate(clinician Clinician) clinicians.Clinician {
	return clinicians.Clinician{
		Name:  clinician.Name,
		Roles: clinician.Roles,
	}
}

func NewPatientDto(patient *patients.Patient) Patient {
	dto := Patient{
		Email:         patient.Email,
		FullName:      pstr(patient.FullName),
		Id:            *strpuseridp(patient.UserId),
		Mrn:           patient.Mrn,
		Permissions:   NewPermissionsDto(patient.Permissions),
		TargetDevices: patient.TargetDevices,
		CreatedTime:   patient.CreatedTime,
		UpdatedTime:   patient.UpdatedTime,
		Summary:       NewSummaryDto(patient.Summary),
	}
	if patient.BirthDate != nil && strtodatep(patient.BirthDate) != nil {
		dto.BirthDate = *strtodatep(patient.BirthDate)
	}
	if !patient.LastUploadReminderTime.IsZero() {
		dto.LastUploadReminderTime = &patient.LastUploadReminderTime
	}
	return dto
}

func NewPatient(dto Patient) patients.Patient {
	return patients.Patient{
		Email:         pstrToLower(dto.Email),
		BirthDate:     strp(dto.BirthDate.Format(dateFormat)),
		FullName:      &dto.FullName,
		Mrn:           dto.Mrn,
		TargetDevices: dto.TargetDevices,
	}
}

func NewSummary(dto *PatientSummary) *patients.Summary {
	if dto == nil {
		return nil
	}

	patientSummary := &patients.Summary{
		FirstData:                dto.FirstData,
		HighGlucoseThreshold:     dto.HighGlucoseThreshold,
		VeryHighGlucoseThreshold: dto.VeryHighGlucoseThreshold,
		LowGlucoseThreshold:      dto.LowGlucoseThreshold,
		VeryLowGlucoseThreshold:  dto.VeryLowGlucoseThreshold,
		LastData:                 dto.LastData,
		LastUpdatedDate:          dto.LastUpdatedDate,
		LastUploadDate:           dto.LastUploadDate,
		HasLastUploadDate:        dto.HasLastUploadDate,
		OutdatedSince:            dto.OutdatedSince,
		TotalHours:               dto.TotalHours,
		Periods:                  map[string]*patients.Period{},
	}

	if dto.Periods != nil {
		// this is bad, but it's better than copy and pasting the copy code N times
		sourcePeriods := map[string]*PatientSummaryPeriod{}
		if dto.Periods.N1d != nil {
			sourcePeriods["1d"] = dto.Periods.N1d
		}
		if dto.Periods.N7d != nil {
			sourcePeriods["7d"] = dto.Periods.N7d
		}
		if dto.Periods.N14d != nil {
			sourcePeriods["14d"] = dto.Periods.N14d
		}
		if dto.Periods.N30d != nil {
			sourcePeriods["30d"] = dto.Periods.N30d
		}

		for i := range sourcePeriods {
			patientSummary.Periods[i] = &patients.Period{
				TimeCGMUsePercent:    sourcePeriods[i].TimeCGMUsePercent,
				HasTimeCGMUsePercent: sourcePeriods[i].HasTimeCGMUsePercent,
				TimeCGMUseMinutes:    sourcePeriods[i].TimeCGMUseMinutes,
				TimeCGMUseRecords:    sourcePeriods[i].TimeCGMUseRecords,

				TimeInVeryLowPercent: sourcePeriods[i].TimeInVeryLowPercent,
				TimeInVeryLowMinutes: sourcePeriods[i].TimeInVeryLowMinutes,
				TimeInVeryLowRecords: sourcePeriods[i].TimeInVeryLowRecords,

				TimeInLowPercent: sourcePeriods[i].TimeInLowPercent,
				TimeInLowMinutes: sourcePeriods[i].TimeInLowMinutes,
				TimeInLowRecords: sourcePeriods[i].TimeInLowRecords,

				TimeInTargetPercent: sourcePeriods[i].TimeInTargetPercent,
				TimeInTargetMinutes: sourcePeriods[i].TimeInTargetMinutes,
				TimeInTargetRecords: sourcePeriods[i].TimeInTargetRecords,

				TimeInHighPercent: sourcePeriods[i].TimeInHighPercent,
				TimeInHighMinutes: sourcePeriods[i].TimeInHighMinutes,
				TimeInHighRecords: sourcePeriods[i].TimeInHighRecords,

				TimeInVeryHighPercent: sourcePeriods[i].TimeInVeryHighPercent,
				TimeInVeryHighMinutes: sourcePeriods[i].TimeInVeryHighMinutes,
				TimeInVeryHighRecords: sourcePeriods[i].TimeInVeryHighRecords,

				GlucoseManagementIndicator:    sourcePeriods[i].GlucoseManagementIndicator,
				HasGlucoseManagementIndicator: sourcePeriods[i].HasGlucoseManagementIndicator,
				AverageGlucose: &patients.AvgGlucose{
					Units: string(sourcePeriods[i].AverageGlucose.Units),
					Value: float64(sourcePeriods[i].AverageGlucose.Value),
				},
			}
		}
	}

	return patientSummary
}

func NewSummaryDto(summary *patients.Summary) *PatientSummary {
	if summary == nil {
		return nil
	}

	patientSummary := &PatientSummary{
		FirstData:            summary.FirstData,
		HighGlucoseThreshold: summary.HighGlucoseThreshold,
		LastData:             summary.LastData,
		LastUpdatedDate:      summary.LastUpdatedDate,
		LastUploadDate:       summary.LastUploadDate,
		HasLastUploadDate:    summary.HasLastUploadDate,
		LowGlucoseThreshold:  summary.LowGlucoseThreshold,
		OutdatedSince:        summary.OutdatedSince,
		TotalHours:           summary.TotalHours,
		Periods:              &PatientSummaryPeriods{},
	}

	if summary.Periods != nil {
		// this is bad, but it's better than copy and pasting the copy code N times
		destPeriods := map[string]*PatientSummaryPeriod{}
		if _, exists := summary.Periods["1d"]; exists {
			patientSummary.Periods.N1d = &PatientSummaryPeriod{}
			destPeriods["1d"] = patientSummary.Periods.N1d
		}
		if _, exists := summary.Periods["7d"]; exists {
			patientSummary.Periods.N7d = &PatientSummaryPeriod{}
			destPeriods["7d"] = patientSummary.Periods.N7d
		}
		if _, exists := summary.Periods["14d"]; exists {
			patientSummary.Periods.N14d = &PatientSummaryPeriod{}
			destPeriods["14d"] = patientSummary.Periods.N14d
		}
		if _, exists := summary.Periods["30d"]; exists {
			patientSummary.Periods.N30d = &PatientSummaryPeriod{}
			destPeriods["30d"] = patientSummary.Periods.N30d
		}

		for i := range destPeriods {
			destPeriods[i].AverageGlucose = &AverageGlucose{
				Value: float32(summary.Periods[i].AverageGlucose.Value),
				Units: AverageGlucoseUnits(summary.Periods[i].AverageGlucose.Units)}

			destPeriods[i].GlucoseManagementIndicator = summary.Periods[i].GlucoseManagementIndicator
			destPeriods[i].HasGlucoseManagementIndicator = summary.Periods[i].HasGlucoseManagementIndicator

			destPeriods[i].TimeCGMUseMinutes = summary.Periods[i].TimeCGMUseMinutes
			destPeriods[i].TimeCGMUsePercent = summary.Periods[i].TimeCGMUsePercent
			destPeriods[i].HasTimeCGMUsePercent = summary.Periods[i].HasTimeCGMUsePercent
			destPeriods[i].TimeCGMUseRecords = summary.Periods[i].TimeCGMUseRecords

			destPeriods[i].TimeInHighMinutes = summary.Periods[i].TimeInHighMinutes
			destPeriods[i].TimeInHighPercent = summary.Periods[i].TimeInHighPercent
			destPeriods[i].TimeInHighRecords = summary.Periods[i].TimeInHighRecords

			destPeriods[i].TimeInLowMinutes = summary.Periods[i].TimeInLowMinutes
			destPeriods[i].TimeInLowPercent = summary.Periods[i].TimeInLowPercent
			destPeriods[i].TimeInLowRecords = summary.Periods[i].TimeInLowRecords

			destPeriods[i].TimeInTargetMinutes = summary.Periods[i].TimeInTargetMinutes
			destPeriods[i].TimeInTargetPercent = summary.Periods[i].TimeInTargetPercent
			destPeriods[i].TimeInTargetRecords = summary.Periods[i].TimeInTargetRecords

			destPeriods[i].TimeInVeryHighMinutes = summary.Periods[i].TimeInVeryHighMinutes
			destPeriods[i].TimeInVeryHighPercent = summary.Periods[i].TimeInVeryHighPercent
			destPeriods[i].TimeInVeryHighRecords = summary.Periods[i].TimeInVeryHighRecords

			destPeriods[i].TimeInVeryLowMinutes = summary.Periods[i].TimeInVeryLowMinutes
			destPeriods[i].TimeInVeryLowPercent = summary.Periods[i].TimeInVeryLowPercent
			destPeriods[i].TimeInVeryLowRecords = summary.Periods[i].TimeInVeryLowRecords
		}
	}

	return patientSummary
}

func NewPermissions(dto *PatientPermissions) *patients.Permissions {
	var permissions *patients.Permissions
	if dto != nil {
		permissions = &patients.Permissions{}
		if dto.Custodian != nil {
			permissions.Custodian = &patients.Permission{}
		}
		if dto.Upload != nil {
			permissions.Upload = &patients.Permission{}
		}
		if dto.Note != nil {
			permissions.Note = &patients.Permission{}
		}
		if dto.View != nil {
			permissions.View = &patients.Permission{}
		}
	}
	return permissions
}

func NewPermissionsDto(dto *patients.Permissions) *PatientPermissions {
	var permissions *PatientPermissions
	if dto != nil {
		permissions = &PatientPermissions{}
		permission := make(map[string]interface{})
		if dto.Custodian != nil {
			permissions.Custodian = &permission
		}
		if dto.Upload != nil {
			permissions.Upload = &permission
		}
		if dto.Note != nil {
			permissions.Note = &permission
		}
		if dto.View != nil {
			permissions.View = &permission
		}
	}
	return permissions
}

func NewPatientsDto(patients []*patients.Patient) []Patient {
	dtos := make([]Patient, 0)
	for _, p := range patients {
		if p != nil {
			dtos = append(dtos, NewPatientDto(p))
		}
	}
	return dtos
}

func NewPatientsResponseDto(list *patients.ListResult) PatientsResponse {
	data := Patients(NewPatientsDto(list.Patients))
	return PatientsResponse{
		Data: &data,
		Meta: &Meta{Count: &list.TotalCount},
	}
}

func NewPatientClinicRelationshipsDto(patients []*patients.Patient, clinicList []*clinics.Clinic) (PatientClinicRelationships, error) {
	clinicsMap := make(map[string]*clinics.Clinic, 0)
	for _, clinic := range clinicList {
		clinicsMap[clinic.Id.Hex()] = clinic
	}
	dtos := make([]PatientClinicRelationship, 0)
	for _, patient := range patients {
		clinic, ok := clinicsMap[patient.ClinicId.Hex()]
		if !ok || clinic == nil {
			return nil, fmt.Errorf("clinic not found")
		}

		dtos = append(dtos, PatientClinicRelationship{
			Clinic:  NewClinicDto(clinic),
			Patient: NewPatientDto(patient),
		})
	}
	return dtos, nil
}

func NewClinicianClinicRelationshipsDto(clinicians []*clinicians.Clinician, clinicList []*clinics.Clinic) (ClinicianClinicRelationships, error) {
	clinicsMap := make(map[string]*clinics.Clinic, 0)
	for _, clinic := range clinicList {
		clinicsMap[clinic.Id.Hex()] = clinic
	}
	dtos := make([]ClinicianClinicRelationship, 0)
	for _, clinician := range clinicians {
		clinic, ok := clinicsMap[clinician.ClinicId.Hex()]
		if !ok || clinic == nil {
			return nil, fmt.Errorf("clinic not found")
		}

		dtos = append(dtos, ClinicianClinicRelationship{
			Clinic:    NewClinicDto(clinic),
			Clinician: NewClinicianDto(clinician),
		})
	}

	return dtos, nil
}

func NewMigrationDto(migration *migration.Migration) *Migration {
	if migration == nil {
		return nil
	}

	result := &Migration{
		CreatedTime: migration.CreatedTime,
		UpdatedTime: migration.UpdatedTime,
		UserId:      migration.UserId,
	}
	if migration.Status != "" {
		status := MigrationStatus(strings.ToUpper(migration.Status))
		result.Status = &status
	}
	return result
}

func NewMigrationDtos(migrations []*migration.Migration) []*Migration {
	var dtos []*Migration
	if len(migrations) == 0 {
		return dtos
	}

	for _, m := range migrations {
		dtos = append(dtos, NewMigrationDto(m))
	}

	return dtos
}

func ParseSort(sort *Sort) ([]*store.Sort, error) {
	if sort == nil {
		return nil, nil
	}
	str := string(*sort)
	var result store.Sort

	if strings.HasPrefix(str, "+") {
		result.Ascending = true
	} else if strings.HasPrefix(str, "-") {
		result.Ascending = false
	} else {
		return nil, fmt.Errorf("%w: invalid sort parameter, missing sort order", errors.BadRequest)
	}

	result.Attribute = str[1:]
	if result.Attribute == "" {
		return nil, fmt.Errorf("%w: invalid sort parameter, missing sort attribute", errors.BadRequest)
	}

	var extraSort = map[string]string{
		"summary.lastUploadDate":                         "summary.hasLastUploadDate",
		"summary.periods.1d.timeCGMUsePercent":           "summary.periods.1d.hasTimeCGMUsePercent",
		"summary.periods.1d.glucoseManagementIndicator":  "summary.periods.1d.hasGlucoseManagementIndicator",
		"summary.periods.7d.timeCGMUsePercent":           "summary.periods.7d.hasTimeCGMUsePercent",
		"summary.periods.7d.glucoseManagementIndicator":  "summary.periods.7d.hasGlucoseManagementIndicator",
		"summary.periods.14d.timeCGMUsePercent":          "summary.periods.14d.hasTimeCGMUsePercent",
		"summary.periods.14d.glucoseManagementIndicator": "summary.periods.14d.hasGlucoseManagementIndicator",
		"summary.periods.30d.timeCGMUsePercent":          "summary.periods.30d.hasTimeCGMUsePercent",
		"summary.periods.30d.glucoseManagementIndicator": "summary.periods.30d.hasGlucoseManagementIndicator",
	}

	var sorts = []*store.Sort{&result}
	if value, exists := extraSort[result.Attribute]; exists {
		sorts = append([]*store.Sort{{Ascending: false, Attribute: value}}, sorts...)
	}

	return sorts, nil
}

const dateFormat = "2006-01-02"

func strtodatep(s *string) *openapi_types.Date {
	if s == nil {
		return nil
	}
	t, err := time.Parse(dateFormat, *s)
	if err != nil {
		return nil
	}
	return &openapi_types.Date{Time: t}
}

func pstr(p *string) string {
	if p == nil {
		return ""
	}

	return *p
}

func strp(s string) *string {
	return &s
}

func strpuseridp(s *string) *TidepoolUserId {
	if s == nil {
		return nil
	}
	id := TidepoolUserId(*s)
	return &id
}

func useridpstrp(u *TidepoolUserId) *string {
	if u == nil {
		return nil
	}
	id := string(*u)
	return &id
}

func searchToString(s *Search) *string {
	if s == nil {
		return nil
	}
	return strp(string(*s))
}

func emailToString(e *Email) *string {
	if e == nil {
		return nil
	}
	return strp(string(*e))
}

func pstrToLower(s *string) *string {
	if s != nil {
		l := strings.ToLower(*s)
		return &l
	}
	return s
}

func roleToString(e *Role) *string {
	if e == nil {
		return nil
	}
	return strp(string(*e))
}

func clinicSizeToString(c *ClinicClinicSize) *string {
	if c == nil {
		return nil
	}
	return strp(string(*c))
}

func stringToClinicSize(s *string) *ClinicClinicSize {
	if s == nil {
		return nil
	}
	size := ClinicClinicSize(*s)
	return &size
}

func clinicTypeToString(c *ClinicClinicType) *string {
	if c == nil {
		return nil
	}
	return strp(string(*c))
}

func stringToClinicType(s *string) *ClinicClinicType {
	if s == nil {
		return nil
	}
	size := ClinicClinicType(*s)
	return &size
}

var rangeFilterRegex = regexp.MustCompile("^(<|<=|>|>=)(\\d\\.\\d?\\d?)$")

func parseRangeFilter(filter string) (cmp *string, val float64, err error) {
	matches := rangeFilterRegex.FindStringSubmatch(filter)
	if len(matches) != 3 {
		err = fmt.Errorf("%w: couldn't parse range filter", errors.BadRequest)
		return
	}
	if _, ok := validCmps[matches[1]]; !ok {
		err = fmt.Errorf("%w: invalid comparator", errors.BadRequest)
		return
	}

	value, e := strconv.ParseFloat(matches[2], 64)
	if e != nil {
		err = fmt.Errorf("%w: invalid value", errors.BadRequest)
		return
	}

	cmp = &matches[1]
	val = value
	err = nil
	return
}

var validCmps = map[string]struct{}{
	">":  {},
	">=": {},
	"<":  {},
	"<=": {},
}
