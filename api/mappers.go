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
		CGM: patients.CGMSummary{
			Periods:           make(map[string]*patients.CGMPeriod),
			TotalHours:        dto.CgmSummary.TotalHours,
			HasLastUploadDate: dto.CgmSummary.HasLastUploadDate,
			LastUploadDate:    dto.CgmSummary.LastUploadDate,
			LastUpdatedDate:   dto.CgmSummary.LastUpdatedDate,
			FirstData:         dto.CgmSummary.FirstData,
			LastData:          dto.CgmSummary.LastData,
			OutdatedSince:     dto.CgmSummary.OutdatedSince,
		},
		BGM: patients.BGMSummary{
			Periods:           make(map[string]*patients.BGMPeriod),
			TotalHours:        dto.BgmSummary.TotalHours,
			HasLastUploadDate: dto.BgmSummary.HasLastUploadDate,
			LastUploadDate:    dto.BgmSummary.LastUploadDate,
			LastUpdatedDate:   dto.BgmSummary.LastUpdatedDate,
			OutdatedSince:     dto.BgmSummary.OutdatedSince,
			FirstData:         dto.BgmSummary.FirstData,
			LastData:          dto.BgmSummary.LastData,
		},
		Config: patients.Config{
			SchemaVersion:            dto.Config.SchemaVersion,
			HighGlucoseThreshold:     dto.Config.HighGlucoseThreshold,
			VeryHighGlucoseThreshold: dto.Config.VeryLowGlucoseThreshold,
			LowGlucoseThreshold:      dto.Config.LowGlucoseThreshold,
			VeryLowGlucoseThreshold:  dto.Config.VeryLowGlucoseThreshold,
		},
	}

	if dto.CgmSummary.Periods != nil {
		var averageGlucose *patients.AverageGlucose
		// this is bad, but it's better than copy and pasting the copy code N times
		sourcePeriods := map[string]*PatientCGMPeriod{}
		if dto.CgmSummary.Periods.N1d != nil {
			sourcePeriods["1d"] = dto.CgmSummary.Periods.N1d
		}
		if dto.CgmSummary.Periods.N7d != nil {
			sourcePeriods["7d"] = dto.CgmSummary.Periods.N7d
		}
		if dto.CgmSummary.Periods.N14d != nil {
			sourcePeriods["14d"] = dto.CgmSummary.Periods.N14d
		}
		if dto.CgmSummary.Periods.N30d != nil {
			sourcePeriods["30d"] = dto.CgmSummary.Periods.N30d
		}

		for i := range sourcePeriods {
			if sourcePeriods[i].AverageGlucose != nil {
				averageGlucose = &patients.AverageGlucose{
					Units: string(sourcePeriods[i].AverageGlucose.Units),
					Value: float64(sourcePeriods[i].AverageGlucose.Value),
				}
			}

			patientSummary.CGM.Periods[i] = &patients.CGMPeriod{
				TimeCGMUsePercent:    sourcePeriods[i].TimeCGMUsePercent,
				HasTimeCGMUsePercent: sourcePeriods[i].HasTimeCGMUsePercent,
				TimeCGMUseMinutes:    sourcePeriods[i].TimeCGMUseMinutes,
				TimeCGMUseRecords:    sourcePeriods[i].TimeCGMUseRecords,

				TimeInVeryLowPercent:    sourcePeriods[i].TimeInVeryLowPercent,
				HasTimeInVeryLowPercent: sourcePeriods[i].HasTimeInVeryLowPercent,
				TimeInVeryLowMinutes:    sourcePeriods[i].TimeInVeryLowMinutes,
				TimeInVeryLowRecords:    sourcePeriods[i].TimeInVeryLowRecords,

				TimeInLowPercent:    sourcePeriods[i].TimeInLowPercent,
				HasTimeInLowPercent: sourcePeriods[i].HasTimeInLowPercent,
				TimeInLowMinutes:    sourcePeriods[i].TimeInLowMinutes,
				TimeInLowRecords:    sourcePeriods[i].TimeInLowRecords,

				TimeInTargetPercent:    sourcePeriods[i].TimeInTargetPercent,
				HasTimeInTargetPercent: sourcePeriods[i].HasTimeInTargetPercent,
				TimeInTargetMinutes:    sourcePeriods[i].TimeInTargetMinutes,
				TimeInTargetRecords:    sourcePeriods[i].TimeInTargetRecords,

				TimeInHighPercent:    sourcePeriods[i].TimeInHighPercent,
				HasTimeInHighPercent: sourcePeriods[i].HasTimeInHighPercent,
				TimeInHighMinutes:    sourcePeriods[i].TimeInHighMinutes,
				TimeInHighRecords:    sourcePeriods[i].TimeInHighRecords,

				TimeInVeryHighPercent:    sourcePeriods[i].TimeInVeryHighPercent,
				HasTimeInVeryHighPercent: sourcePeriods[i].HasTimeInVeryHighPercent,
				TimeInVeryHighMinutes:    sourcePeriods[i].TimeInVeryHighMinutes,
				TimeInVeryHighRecords:    sourcePeriods[i].TimeInVeryHighRecords,

				GlucoseManagementIndicator:    sourcePeriods[i].GlucoseManagementIndicator,
				HasGlucoseManagementIndicator: sourcePeriods[i].HasGlucoseManagementIndicator,
				AverageGlucose:                averageGlucose,
				HasAverageGlucose:             sourcePeriods[i].HasAverageGlucose,
			}
		}
	}

	if dto.BgmSummary.Periods != nil {
		var averageGlucose *patients.AverageGlucose
		// this is bad, but it's better than copy and pasting the copy code N times
		sourcePeriods := map[string]*PatientBGMPeriod{}
		if dto.CgmSummary.Periods.N1d != nil {
			sourcePeriods["1d"] = dto.BgmSummary.Periods.N1d
		}
		if dto.CgmSummary.Periods.N7d != nil {
			sourcePeriods["7d"] = dto.BgmSummary.Periods.N7d
		}
		if dto.CgmSummary.Periods.N14d != nil {
			sourcePeriods["14d"] = dto.BgmSummary.Periods.N14d
		}
		if dto.CgmSummary.Periods.N30d != nil {
			sourcePeriods["30d"] = dto.BgmSummary.Periods.N30d
		}

		for i := range sourcePeriods {
			if sourcePeriods[i].AverageGlucose != nil {
				averageGlucose = &patients.AverageGlucose{
					Units: string(sourcePeriods[i].AverageGlucose.Units),
					Value: float64(sourcePeriods[i].AverageGlucose.Value),
				}
			}

			patientSummary.CGM.Periods[i] = &patients.CGMPeriod{
				TimeInVeryLowPercent:    sourcePeriods[i].TimeInVeryLowPercent,
				HasTimeInVeryLowPercent: sourcePeriods[i].HasTimeInVeryLowPercent,
				TimeInVeryLowRecords:    sourcePeriods[i].TimeInVeryLowRecords,

				TimeInLowPercent:    sourcePeriods[i].TimeInLowPercent,
				HasTimeInLowPercent: sourcePeriods[i].HasTimeInLowPercent,
				TimeInLowRecords:    sourcePeriods[i].TimeInLowRecords,

				TimeInTargetPercent:    sourcePeriods[i].TimeInTargetPercent,
				HasTimeInTargetPercent: sourcePeriods[i].HasTimeInTargetPercent,
				TimeInTargetRecords:    sourcePeriods[i].TimeInTargetRecords,

				TimeInHighPercent:    sourcePeriods[i].TimeInHighPercent,
				HasTimeInHighPercent: sourcePeriods[i].HasTimeInHighPercent,
				TimeInHighRecords:    sourcePeriods[i].TimeInHighRecords,

				TimeInVeryHighPercent:    sourcePeriods[i].TimeInVeryHighPercent,
				HasTimeInVeryHighPercent: sourcePeriods[i].HasTimeInVeryHighPercent,
				TimeInVeryHighRecords:    sourcePeriods[i].TimeInVeryHighRecords,

				AverageGlucose:    averageGlucose,
				HasAverageGlucose: sourcePeriods[i].HasAverageGlucose,
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
		BgmSummary: &PatientBGMSummary{
			FirstData:         summary.BGM.FirstData,
			HasLastUploadDate: summary.BGM.HasLastUploadDate,
			LastData:          summary.BGM.LastData,
			LastUpdatedDate:   summary.BGM.LastUpdatedDate,
			LastUploadDate:    summary.BGM.LastUploadDate,
			OutdatedSince:     summary.BGM.OutdatedSince,
			Periods:           &PatientBGMPeriods{},
			TotalHours:        summary.BGM.TotalHours,
		},
		CgmSummary: &PatientCGMSummary{
			FirstData:         summary.CGM.FirstData,
			HasLastUploadDate: summary.CGM.HasLastUploadDate,
			LastData:          summary.CGM.LastData,
			LastUpdatedDate:   summary.CGM.LastUpdatedDate,
			LastUploadDate:    summary.CGM.LastUploadDate,
			OutdatedSince:     summary.CGM.OutdatedSince,
			Periods:           &PatientCGMPeriods{},
			TotalHours:        summary.CGM.TotalHours,
		},
		Config: &PatientSummaryConfig{
			SchemaVersion:            summary.Config.SchemaVersion,
			HighGlucoseThreshold:     summary.Config.HighGlucoseThreshold,
			VeryHighGlucoseThreshold: summary.Config.VeryHighGlucoseThreshold,
			LowGlucoseThreshold:      summary.Config.LowGlucoseThreshold,
			VeryLowGlucoseThreshold:  summary.Config.VeryLowGlucoseThreshold,
		},
	}

	if summary.CGM.Periods != nil {
		// this is bad, but it's better than copy and pasting the copy code N times
		destPeriods := map[string]*PatientCGMPeriod{}
		if _, exists := summary.CGM.Periods["1d"]; exists {
			patientSummary.CgmSummary.Periods.N1d = &PatientCGMPeriod{}
			destPeriods["1d"] = patientSummary.CgmSummary.Periods.N1d
		}
		if _, exists := summary.CGM.Periods["7d"]; exists {
			patientSummary.CgmSummary.Periods.N7d = &PatientCGMPeriod{}
			destPeriods["7d"] = patientSummary.CgmSummary.Periods.N7d
		}
		if _, exists := summary.CGM.Periods["14d"]; exists {
			patientSummary.CgmSummary.Periods.N14d = &PatientCGMPeriod{}
			destPeriods["14d"] = patientSummary.CgmSummary.Periods.N14d
		}
		if _, exists := summary.CGM.Periods["30d"]; exists {
			patientSummary.CgmSummary.Periods.N30d = &PatientCGMPeriod{}
			destPeriods["30d"] = patientSummary.CgmSummary.Periods.N30d
		}

		for i := range destPeriods {
			if summary.CGM.Periods[i].AverageGlucose != nil {
				destPeriods[i].AverageGlucose = &AverageGlucose{
					Value: float32(summary.CGM.Periods[i].AverageGlucose.Value),
					Units: AverageGlucoseUnits(summary.CGM.Periods[i].AverageGlucose.Units)}
			}
			destPeriods[i].HasAverageGlucose = summary.CGM.Periods[i].HasAverageGlucose

			destPeriods[i].GlucoseManagementIndicator = summary.CGM.Periods[i].GlucoseManagementIndicator
			destPeriods[i].HasGlucoseManagementIndicator = summary.CGM.Periods[i].HasGlucoseManagementIndicator

			destPeriods[i].TimeCGMUseMinutes = summary.CGM.Periods[i].TimeCGMUseMinutes
			destPeriods[i].TimeCGMUsePercent = summary.CGM.Periods[i].TimeCGMUsePercent
			destPeriods[i].HasTimeCGMUsePercent = summary.CGM.Periods[i].HasTimeCGMUsePercent
			destPeriods[i].TimeCGMUseRecords = summary.CGM.Periods[i].TimeCGMUseRecords

			destPeriods[i].TimeInHighMinutes = summary.CGM.Periods[i].TimeInHighMinutes
			destPeriods[i].TimeInHighPercent = summary.CGM.Periods[i].TimeInHighPercent
			destPeriods[i].HasTimeInHighPercent = summary.CGM.Periods[i].HasTimeInHighPercent
			destPeriods[i].TimeInHighRecords = summary.CGM.Periods[i].TimeInHighRecords

			destPeriods[i].TimeInLowMinutes = summary.CGM.Periods[i].TimeInLowMinutes
			destPeriods[i].TimeInLowPercent = summary.CGM.Periods[i].TimeInLowPercent
			destPeriods[i].HasTimeInLowPercent = summary.CGM.Periods[i].HasTimeInLowPercent
			destPeriods[i].TimeInLowRecords = summary.CGM.Periods[i].TimeInLowRecords

			destPeriods[i].TimeInTargetMinutes = summary.CGM.Periods[i].TimeInTargetMinutes
			destPeriods[i].TimeInTargetPercent = summary.CGM.Periods[i].TimeInTargetPercent
			destPeriods[i].HasTimeInTargetPercent = summary.CGM.Periods[i].HasTimeInTargetPercent
			destPeriods[i].TimeInTargetRecords = summary.CGM.Periods[i].TimeInTargetRecords

			destPeriods[i].TimeInVeryHighMinutes = summary.CGM.Periods[i].TimeInVeryHighMinutes
			destPeriods[i].TimeInVeryHighPercent = summary.CGM.Periods[i].TimeInVeryHighPercent
			destPeriods[i].HasTimeInVeryHighPercent = summary.CGM.Periods[i].HasTimeInVeryHighPercent
			destPeriods[i].TimeInVeryHighRecords = summary.CGM.Periods[i].TimeInVeryHighRecords

			destPeriods[i].TimeInVeryLowMinutes = summary.CGM.Periods[i].TimeInVeryLowMinutes
			destPeriods[i].TimeInVeryLowPercent = summary.CGM.Periods[i].TimeInVeryLowPercent
			destPeriods[i].HasTimeInVeryLowPercent = summary.CGM.Periods[i].HasTimeInVeryLowPercent
			destPeriods[i].TimeInVeryLowRecords = summary.CGM.Periods[i].TimeInVeryLowRecords
		}
	}

	if summary.BGM.Periods != nil {
		// this is bad, but it's better than copy and pasting the copy code N times
		destPeriods := map[string]*PatientBGMPeriod{}
		if _, exists := summary.BGM.Periods["1d"]; exists {
			patientSummary.BgmSummary.Periods.N1d = &PatientBGMPeriod{}
			destPeriods["1d"] = patientSummary.BgmSummary.Periods.N1d
		}
		if _, exists := summary.BGM.Periods["7d"]; exists {
			patientSummary.BgmSummary.Periods.N7d = &PatientBGMPeriod{}
			destPeriods["7d"] = patientSummary.BgmSummary.Periods.N7d
		}
		if _, exists := summary.BGM.Periods["14d"]; exists {
			patientSummary.BgmSummary.Periods.N14d = &PatientBGMPeriod{}
			destPeriods["14d"] = patientSummary.BgmSummary.Periods.N14d
		}
		if _, exists := summary.BGM.Periods["30d"]; exists {
			patientSummary.BgmSummary.Periods.N30d = &PatientBGMPeriod{}
			destPeriods["30d"] = patientSummary.BgmSummary.Periods.N30d
		}

		for i := range destPeriods {
			if summary.BGM.Periods[i].AverageGlucose != nil {
				destPeriods[i].AverageGlucose = &AverageGlucose{
					Value: float32(summary.BGM.Periods[i].AverageGlucose.Value),
					Units: AverageGlucoseUnits(summary.BGM.Periods[i].AverageGlucose.Units)}
			}
			destPeriods[i].HasAverageGlucose = summary.BGM.Periods[i].HasAverageGlucose

			destPeriods[i].TimeInHighPercent = summary.BGM.Periods[i].TimeInHighPercent
			destPeriods[i].HasTimeInHighPercent = summary.BGM.Periods[i].HasTimeInHighPercent
			destPeriods[i].TimeInHighRecords = summary.BGM.Periods[i].TimeInHighRecords

			destPeriods[i].TimeInLowPercent = summary.BGM.Periods[i].TimeInLowPercent
			destPeriods[i].HasTimeInLowPercent = summary.BGM.Periods[i].HasTimeInLowPercent
			destPeriods[i].TimeInLowRecords = summary.BGM.Periods[i].TimeInLowRecords

			destPeriods[i].TimeInTargetPercent = summary.BGM.Periods[i].TimeInTargetPercent
			destPeriods[i].HasTimeInTargetPercent = summary.BGM.Periods[i].HasTimeInTargetPercent
			destPeriods[i].TimeInTargetRecords = summary.BGM.Periods[i].TimeInTargetRecords

			destPeriods[i].TimeInVeryHighPercent = summary.BGM.Periods[i].TimeInVeryHighPercent
			destPeriods[i].HasTimeInVeryHighPercent = summary.BGM.Periods[i].HasTimeInVeryHighPercent
			destPeriods[i].TimeInVeryHighRecords = summary.BGM.Periods[i].TimeInVeryHighRecords

			destPeriods[i].TimeInVeryLowPercent = summary.BGM.Periods[i].TimeInVeryLowPercent
			destPeriods[i].HasTimeInVeryLowPercent = summary.BGM.Periods[i].HasTimeInVeryLowPercent
			destPeriods[i].TimeInVeryLowRecords = summary.BGM.Periods[i].TimeInVeryLowRecords
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
		"summary.lastUploadDate": "summary.hasLastUploadDate",

		"summary.periods.1d.timeCGMUsePercent":          "summary.periods.1d.hasTimeCGMUsePercent",
		"summary.periods.1d.glucoseManagementIndicator": "summary.periods.1d.hasGlucoseManagementIndicator",
		"summary.periods.1d.averageGlucose.value":       "summary.periods.1d.hasAverageGlucose",
		"summary.periods.1d.timeInTargetPercent":        "summary.periods.1d.hasTimeInTargetPercent",
		"summary.periods.1d.timeInLowPercent":           "summary.periods.1d.hasTimeInLowPercent",
		"summary.periods.1d.timeInVeryLowPercent":       "summary.periods.1d.hasTimeInVeryLowPercent",
		"summary.periods.1d.timeInHighPercent":          "summary.periods.1d.hasTimeInHighPercent",
		"summary.periods.1d.timeInVeryHighPercent":      "summary.periods.1d.hasTimeInVeryHighPercent",

		"summary.periods.7d.timeCGMUsePercent":          "summary.periods.7d.hasTimeCGMUsePercent",
		"summary.periods.7d.glucoseManagementIndicator": "summary.periods.7d.hasGlucoseManagementIndicator",
		"summary.periods.7d.averageGlucose.value":       "summary.periods.7d.hasAverageGlucose",
		"summary.periods.7d.timeInTargetPercent":        "summary.periods.7d.hasTimeInTargetPercent",
		"summary.periods.7d.timeInLowPercent":           "summary.periods.7d.hasTimeInLowPercent",
		"summary.periods.7d.timeInVeryLowPercent":       "summary.periods.7d.hasTimeInVeryLowPercent",
		"summary.periods.7d.timeInHighPercent":          "summary.periods.7d.hasTimeInHighPercent",
		"summary.periods.7d.timeInVeryHighPercent":      "summary.periods.7d.hasTimeInVeryHighPercent",

		"summary.periods.14d.timeCGMUsePercent":          "summary.periods.14d.hasTimeCGMUsePercent",
		"summary.periods.14d.glucoseManagementIndicator": "summary.periods.14d.hasGlucoseManagementIndicator",
		"summary.periods.14d.averageGlucose.value":       "summary.periods.14d.hasAverageGlucose",
		"summary.periods.14d.timeInTargetPercent":        "summary.periods.14d.hasTimeInTargetPercent",
		"summary.periods.14d.timeInLowPercent":           "summary.periods.14d.hasTimeInLowPercent",
		"summary.periods.14d.timeInVeryLowPercent":       "summary.periods.14d.hasTimeInVeryLowPercent",
		"summary.periods.14d.timeInHighPercent":          "summary.periods.14d.hasTimeInHighPercent",
		"summary.periods.14d.timeInVeryHighPercent":      "summary.periods.14d.hasTimeInVeryHighPercent",

		"summary.periods.30d.timeCGMUsePercent":          "summary.periods.30d.hasTimeCGMUsePercent",
		"summary.periods.30d.glucoseManagementIndicator": "summary.periods.30d.hasGlucoseManagementIndicator",
		"summary.periods.30d.averageGlucose.value":       "summary.periods.30d.hasAverageGlucose",
		"summary.periods.30d.timeInTargetPercent":        "summary.periods.30d.hasTimeInTargetPercent",
		"summary.periods.30d.timeInLowPercent":           "summary.periods.30d.hasTimeInLowPercent",
		"summary.periods.30d.timeInVeryLowPercent":       "summary.periods.30d.hasTimeInVeryLowPercent",
		"summary.periods.30d.timeInHighPercent":          "summary.periods.30d.hasTimeInHighPercent",
		"summary.periods.30d.timeInVeryHighPercent":      "summary.periods.30d.hasTimeInVeryHighPercent",
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
