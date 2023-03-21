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

	patientSummary := &patients.Summary{}

	if dto.CgmStats != nil {
		patientSummary.CGM = &patients.CGMStats{
			Periods:    make(map[string]*patients.CGMPeriod),
			TotalHours: dto.CgmStats.TotalHours,
			Config: patients.Config{
				SchemaVersion:            dto.CgmStats.Config.SchemaVersion,
				HighGlucoseThreshold:     dto.CgmStats.Config.HighGlucoseThreshold,
				VeryHighGlucoseThreshold: dto.CgmStats.Config.VeryLowGlucoseThreshold,
				LowGlucoseThreshold:      dto.CgmStats.Config.LowGlucoseThreshold,
				VeryLowGlucoseThreshold:  dto.CgmStats.Config.VeryLowGlucoseThreshold,
			},
			Dates: patients.Dates{
				HasLastUploadDate: dto.CgmStats.Dates.HasLastUploadDate,
				LastUploadDate:    dto.CgmStats.Dates.LastUploadDate,
				LastUpdatedDate:   dto.CgmStats.Dates.LastUpdatedDate,
				OutdatedSince:     dto.CgmStats.Dates.OutdatedSince,
				FirstData:         dto.CgmStats.Dates.FirstData,
				LastData:          dto.CgmStats.Dates.LastData,
			},
		}

		if dto.CgmStats.Periods != nil {
			var averageGlucose *patients.AverageGlucose
			// this is bad, but it's better than copy and pasting the copy code N times
			sourcePeriods := map[string]*PatientCGMPeriod{}
			if dto.CgmStats.Periods.N1d != nil {
				sourcePeriods["1d"] = dto.CgmStats.Periods.N1d
			}
			if dto.CgmStats.Periods.N7d != nil {
				sourcePeriods["7d"] = dto.CgmStats.Periods.N7d
			}
			if dto.CgmStats.Periods.N14d != nil {
				sourcePeriods["14d"] = dto.CgmStats.Periods.N14d
			}
			if dto.CgmStats.Periods.N30d != nil {
				sourcePeriods["30d"] = dto.CgmStats.Periods.N30d
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
	}

	if dto.BgmStats != nil {
		patientSummary.BGM = &patients.BGMStats{
			Periods:    make(map[string]*patients.BGMPeriod),
			TotalHours: dto.BgmStats.TotalHours,
			Config: patients.Config{
				SchemaVersion:            dto.BgmStats.Config.SchemaVersion,
				HighGlucoseThreshold:     dto.BgmStats.Config.HighGlucoseThreshold,
				VeryHighGlucoseThreshold: dto.BgmStats.Config.VeryLowGlucoseThreshold,
				LowGlucoseThreshold:      dto.BgmStats.Config.LowGlucoseThreshold,
				VeryLowGlucoseThreshold:  dto.BgmStats.Config.VeryLowGlucoseThreshold,
			},
			Dates: patients.Dates{
				HasLastUploadDate: dto.BgmStats.Dates.HasLastUploadDate,
				LastUploadDate:    dto.BgmStats.Dates.LastUploadDate,
				LastUpdatedDate:   dto.BgmStats.Dates.LastUpdatedDate,
				OutdatedSince:     dto.BgmStats.Dates.OutdatedSince,
				FirstData:         dto.BgmStats.Dates.FirstData,
				LastData:          dto.BgmStats.Dates.LastData,
			},
		}

		if dto.BgmStats.Periods != nil {
			var averageGlucose *patients.AverageGlucose
			// this is bad, but it's better than copy and pasting the copy code N times
			sourcePeriods := map[string]*PatientBGMPeriod{}
			if dto.BgmStats.Periods.N1d != nil {
				sourcePeriods["1d"] = dto.BgmStats.Periods.N1d
			}
			if dto.BgmStats.Periods.N7d != nil {
				sourcePeriods["7d"] = dto.BgmStats.Periods.N7d
			}
			if dto.BgmStats.Periods.N14d != nil {
				sourcePeriods["14d"] = dto.BgmStats.Periods.N14d
			}
			if dto.BgmStats.Periods.N30d != nil {
				sourcePeriods["30d"] = dto.BgmStats.Periods.N30d
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
	}

	return patientSummary
}

func NewSummaryDto(summary *patients.Summary) *PatientSummary {
	if summary == nil {
		return nil
	}

	patientSummary := &PatientSummary{}

	if summary.CGM != nil {
		patientSummary.CgmStats = &PatientCGMStats{
			Config: &PatientSummaryConfig{
				SchemaVersion:            summary.CGM.Config.SchemaVersion,
				HighGlucoseThreshold:     summary.CGM.Config.HighGlucoseThreshold,
				VeryHighGlucoseThreshold: summary.CGM.Config.VeryHighGlucoseThreshold,
				LowGlucoseThreshold:      summary.CGM.Config.LowGlucoseThreshold,
				VeryLowGlucoseThreshold:  summary.CGM.Config.VeryLowGlucoseThreshold,
			},
			Dates: &PatientSummaryDates{
				FirstData:         summary.CGM.Dates.FirstData,
				HasLastUploadDate: summary.CGM.Dates.HasLastUploadDate,
				LastData:          summary.CGM.Dates.LastData,
				LastUpdatedDate:   summary.CGM.Dates.LastUpdatedDate,
				LastUploadDate:    summary.CGM.Dates.LastUploadDate,
				OutdatedSince:     summary.CGM.Dates.OutdatedSince,
			},
			Periods:    &PatientCGMPeriods{},
			TotalHours: summary.CGM.TotalHours,
		}

		if summary.CGM.Periods != nil {
			// this is bad, but it's better than copy and pasting the copy code N times
			destPeriods := map[string]*PatientCGMPeriod{}
			if _, exists := summary.CGM.Periods["1d"]; exists {
				patientSummary.CgmStats.Periods.N1d = &PatientCGMPeriod{}
				destPeriods["1d"] = patientSummary.CgmStats.Periods.N1d
			}
			if _, exists := summary.CGM.Periods["7d"]; exists {
				patientSummary.CgmStats.Periods.N7d = &PatientCGMPeriod{}
				destPeriods["7d"] = patientSummary.CgmStats.Periods.N7d
			}
			if _, exists := summary.CGM.Periods["14d"]; exists {
				patientSummary.CgmStats.Periods.N14d = &PatientCGMPeriod{}
				destPeriods["14d"] = patientSummary.CgmStats.Periods.N14d
			}
			if _, exists := summary.CGM.Periods["30d"]; exists {
				patientSummary.CgmStats.Periods.N30d = &PatientCGMPeriod{}
				destPeriods["30d"] = patientSummary.CgmStats.Periods.N30d
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
	}

	if summary.BGM != nil {
		patientSummary.BgmStats = &PatientBGMStats{
			Config: &PatientSummaryConfig{
				SchemaVersion:            summary.BGM.Config.SchemaVersion,
				HighGlucoseThreshold:     summary.BGM.Config.HighGlucoseThreshold,
				VeryHighGlucoseThreshold: summary.BGM.Config.VeryHighGlucoseThreshold,
				LowGlucoseThreshold:      summary.BGM.Config.LowGlucoseThreshold,
				VeryLowGlucoseThreshold:  summary.BGM.Config.VeryLowGlucoseThreshold,
			},
			Dates: &PatientSummaryDates{
				FirstData:         summary.BGM.Dates.FirstData,
				HasLastUploadDate: summary.BGM.Dates.HasLastUploadDate,
				LastData:          summary.BGM.Dates.LastData,
				LastUpdatedDate:   summary.BGM.Dates.LastUpdatedDate,
				LastUploadDate:    summary.BGM.Dates.LastUploadDate,
				OutdatedSince:     summary.BGM.Dates.OutdatedSince,
			},
			Periods:    &PatientBGMPeriods{},
			TotalHours: summary.BGM.TotalHours,
		}

		if summary.BGM.Periods != nil {
			// this is bad, but it's better than copy and pasting the copy code N times
			destPeriods := map[string]*PatientBGMPeriod{}
			if _, exists := summary.BGM.Periods["1d"]; exists {
				patientSummary.BgmStats.Periods.N1d = &PatientBGMPeriod{}
				destPeriods["1d"] = patientSummary.BgmStats.Periods.N1d
			}
			if _, exists := summary.BGM.Periods["7d"]; exists {
				patientSummary.BgmStats.Periods.N7d = &PatientBGMPeriod{}
				destPeriods["7d"] = patientSummary.BgmStats.Periods.N7d
			}
			if _, exists := summary.BGM.Periods["14d"]; exists {
				patientSummary.BgmStats.Periods.N14d = &PatientBGMPeriod{}
				destPeriods["14d"] = patientSummary.BgmStats.Periods.N14d
			}
			if _, exists := summary.BGM.Periods["30d"]; exists {
				patientSummary.BgmStats.Periods.N30d = &PatientBGMPeriod{}
				destPeriods["30d"] = patientSummary.BgmStats.Periods.N30d
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

func ParseSort(sort *Sort, t string, period string) ([]*store.Sort, error) {
	if sort == nil {
		return nil, nil
	}
	str := string(*sort)
	var result store.Sort

	if t == "" {
		return nil, fmt.Errorf("%w: invalid sort parameter, missing type", errors.BadRequest)
	} else if t != "cgm" && t != "bgm" {
		return nil, fmt.Errorf("%w: invalid sort parameter, invalid type", errors.BadRequest)
	}

	if period == "" {
		return nil, fmt.Errorf("%w: invalid sort parameter, missing period", errors.BadRequest)
	} else if period != "1d" && period != "7d" && period != "14d" && period != "30d" {
		return nil, fmt.Errorf("%w: invalid sort parameter, invalid period", errors.BadRequest)
	}

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
	} else if !isSortAttributeValid(result.Attribute, t) {
		return nil, fmt.Errorf("%w: invalid sort parameter, invalid sort attribute", errors.BadRequest)
	}

	var expandedSorts = map[string]string{
		"lastUploadDate":             "summary." + t + "Stats.dates.lastUploadDate",
		"averageGlucose":             "summary." + t + "Stats.periods." + period + ".averageGlucose.value",
		"timeCGMUsePercent":          "summary." + t + "Stats.periods." + period + ".timeCGMUsePercent",
		"glucoseManagementIndicator": "summary." + t + "Stats.periods." + period + ".glucoseManagementIndicator",
		"timeInTargetPercent":        "summary." + t + "Stats.periods." + period + ".timeInTargetPercent",
		"timeInLowPercent":           "summary." + t + "Stats.periods." + period + ".timeInLowPercent",
		"timeInVeryLowPercent":       "summary." + t + "Stats.periods." + period + ".timeInVeryLowPercent",
		"timeInHighPercent":          "summary." + t + "Stats.periods." + period + ".timeInHighPercent",
		"timeInVeryHighPercent":      "summary." + t + "Stats.periods." + period + ".timeInVeryHighPercent",
		"averageDailyRecords":        "summary." + t + "Stats.periods." + period + ".averageDailyRecords",

		"hasLastUploadDate":             "summary." + t + "Stats.periods." + period + ".hasLastUploadDate",
		"hasTimeCGMUsePercent":          "summary." + t + "Stats.periods." + period + ".hasTimeCGMUsePercent",
		"hasGlucoseManagementIndicator": "summary." + t + "Stats.periods." + period + ".hasGlucoseManagementIndicator",
		"hasAverageGlucose":             "summary." + t + "Stats.periods." + period + ".hasAverageGlucose",
		"hasTimeInTargetPercent":        "summary." + t + "Stats.periods." + period + ".hasTimeInTargetPercent",
		"hasTimeInLowPercent":           "summary." + t + "Stats.periods." + period + ".hasTimeInLowPercent",
		"hasTimeInVeryLowPercent":       "summary." + t + "Stats.periods." + period + ".hasTimeInVeryLowPercent",
		"hasTimeInHighPercent":          "summary." + t + "Stats.periods." + period + ".hasTimeInHighPercent",
		"hasTimeInVeryHighPercent":      "summary." + t + "Stats.periods." + period + ".hasTimeInVeryHighPercent",
	}

	var extraSort = map[string]string{
		expandedSorts["lastUploadDate"]:             expandedSorts["hasLastUploadDate"],
		expandedSorts["timeCGMUsePercent"]:          expandedSorts["hasTimeCGMUsePercent"],
		expandedSorts["glucoseManagementIndicator"]: expandedSorts["hasGlucoseManagementIndicator"],
		expandedSorts["averageGlucose"]:             expandedSorts["hasAverageGlucose"],
		expandedSorts["timeInTargetPercent"]:        expandedSorts["hasTimeInTargetPercent"],
		expandedSorts["timeInLowPercent"]:           expandedSorts["hasTimeInLowPercent"],
		expandedSorts["timeInVeryLowPercent"]:       expandedSorts["hasTimeInVeryLowPercent"],
		expandedSorts["timeInHighPercent"]:          expandedSorts["hasTimeInHighPercent"],
		expandedSorts["timeInVeryHighPercent"]:      expandedSorts["hasTimeInVeryHighPercent"],
	}

	// expand the original param now that we are done using it as a map key
	if value, exists := expandedSorts[result.Attribute]; exists {
		result.Attribute = value
	}

	// add any extra sort keys needed for a key to work as intended (empty always last)
	var sorts = []*store.Sort{&result}
	if value, exists := extraSort[result.Attribute]; exists {
		sorts = append([]*store.Sort{{Ascending: false, Attribute: value}}, sorts...)
	}

	return sorts, nil
}

var validSortAttributes = map[string]map[string]struct{}{
	"cgm": {
		"fullName":       {},
		"birthDate":      {},
		"lastUploadDate": {},

		"timeCGMUsePercent":          {},
		"hasTimeCGMUsePercent":       {},
		"glucoseManagementIndicator": {},
		"averageGlucose":             {},
		"timeInLowPercent":           {},
		"timeInVeryLowPercent":       {},
		"timeInHighPercent":          {},
		"timeInVeryHighPercent":      {},
		"timeInTargetPercent":        {},
		"averageDailyRecords":        {},
	},
	"bgm": {
		"fullName":       {},
		"birthDate":      {},
		"lastUploadDate": {},

		"averageGlucose":        {},
		"timeInLowPercent":      {},
		"timeInVeryLowPercent":  {},
		"timeInHighPercent":     {},
		"timeInVeryHighPercent": {},
		"timeInTargetPercent":   {},
		"averageDailyRecords":   {},
	},
}

func isSortAttributeValid(attribute string, t string) bool {
	_, ok := validSortAttributes[t][attribute]
	return ok
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
