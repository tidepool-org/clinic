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
		Periods:                  make(map[string]*patients.Period),
	}

	if dto.Periods != nil {
		if dto.Periods.N1d != nil {
			patientSummary.Periods["1d"] = &patients.Period{
				TimeCGMUsePercent:    dto.Periods.N1d.TimeCGMUsePercent,
				HasTimeCGMUsePercent: dto.Periods.N1d.HasTimeCGMUsePercent,
				TimeCGMUseMinutes:    dto.Periods.N1d.TimeCGMUseMinutes,
				TimeCGMUseRecords:    dto.Periods.N1d.TimeCGMUseRecords,

				TimeInVeryLowPercent: dto.Periods.N1d.TimeInVeryLowPercent,
				TimeInVeryLowMinutes: dto.Periods.N1d.TimeInVeryLowMinutes,
				TimeInVeryLowRecords: dto.Periods.N1d.TimeInVeryLowRecords,

				TimeInLowPercent: dto.Periods.N1d.TimeInLowPercent,
				TimeInLowMinutes: dto.Periods.N1d.TimeInLowMinutes,
				TimeInLowRecords: dto.Periods.N1d.TimeInLowRecords,

				TimeInTargetPercent: dto.Periods.N1d.TimeInTargetPercent,
				TimeInTargetMinutes: dto.Periods.N1d.TimeInTargetMinutes,
				TimeInTargetRecords: dto.Periods.N1d.TimeInTargetRecords,

				TimeInHighPercent: dto.Periods.N1d.TimeInHighPercent,
				TimeInHighMinutes: dto.Periods.N1d.TimeInHighMinutes,
				TimeInHighRecords: dto.Periods.N1d.TimeInHighRecords,

				TimeInVeryHighPercent: dto.Periods.N1d.TimeInVeryHighPercent,
				TimeInVeryHighMinutes: dto.Periods.N1d.TimeInVeryHighMinutes,
				TimeInVeryHighRecords: dto.Periods.N1d.TimeInVeryHighRecords,

				GlucoseManagementIndicator:    dto.Periods.N1d.GlucoseManagementIndicator,
				HasGlucoseManagementIndicator: dto.Periods.N1d.HasGlucoseManagementIndicator,
				AverageGlucose: &patients.AvgGlucose{
					Units: string(dto.Periods.N1d.AverageGlucose.Units),
					Value: float64(dto.Periods.N1d.AverageGlucose.Value),
				},
			}
		}

		if dto.Periods.N7d != nil {
			patientSummary.Periods["7d"] = &patients.Period{
				TimeCGMUsePercent:    dto.Periods.N7d.TimeCGMUsePercent,
				HasTimeCGMUsePercent: dto.Periods.N7d.HasTimeCGMUsePercent,
				TimeCGMUseMinutes:    dto.Periods.N7d.TimeCGMUseMinutes,
				TimeCGMUseRecords:    dto.Periods.N7d.TimeCGMUseRecords,

				TimeInVeryLowPercent: dto.Periods.N7d.TimeInVeryLowPercent,
				TimeInVeryLowMinutes: dto.Periods.N7d.TimeInVeryLowMinutes,
				TimeInVeryLowRecords: dto.Periods.N7d.TimeInVeryLowRecords,

				TimeInLowPercent: dto.Periods.N7d.TimeInLowPercent,
				TimeInLowMinutes: dto.Periods.N7d.TimeInLowMinutes,
				TimeInLowRecords: dto.Periods.N7d.TimeInLowRecords,

				TimeInTargetPercent: dto.Periods.N7d.TimeInTargetPercent,
				TimeInTargetMinutes: dto.Periods.N7d.TimeInTargetMinutes,
				TimeInTargetRecords: dto.Periods.N7d.TimeInTargetRecords,

				TimeInHighPercent: dto.Periods.N7d.TimeInHighPercent,
				TimeInHighMinutes: dto.Periods.N7d.TimeInHighMinutes,
				TimeInHighRecords: dto.Periods.N7d.TimeInHighRecords,

				TimeInVeryHighPercent: dto.Periods.N7d.TimeInVeryHighPercent,
				TimeInVeryHighMinutes: dto.Periods.N7d.TimeInVeryHighMinutes,
				TimeInVeryHighRecords: dto.Periods.N7d.TimeInVeryHighRecords,

				GlucoseManagementIndicator:    dto.Periods.N7d.GlucoseManagementIndicator,
				HasGlucoseManagementIndicator: dto.Periods.N7d.HasGlucoseManagementIndicator,
				AverageGlucose: &patients.AvgGlucose{
					Units: string(dto.Periods.N7d.AverageGlucose.Units),
					Value: float64(dto.Periods.N7d.AverageGlucose.Value),
				},
			}
		}

		if dto.Periods.N14d != nil {
			patientSummary.Periods["14d"] = &patients.Period{
				TimeCGMUsePercent:    dto.Periods.N14d.TimeCGMUsePercent,
				HasTimeCGMUsePercent: dto.Periods.N14d.HasTimeCGMUsePercent,
				TimeCGMUseMinutes:    dto.Periods.N14d.TimeCGMUseMinutes,
				TimeCGMUseRecords:    dto.Periods.N14d.TimeCGMUseRecords,

				TimeInVeryLowPercent: dto.Periods.N14d.TimeInVeryLowPercent,
				TimeInVeryLowMinutes: dto.Periods.N14d.TimeInVeryLowMinutes,
				TimeInVeryLowRecords: dto.Periods.N14d.TimeInVeryLowRecords,

				TimeInLowPercent: dto.Periods.N14d.TimeInLowPercent,
				TimeInLowMinutes: dto.Periods.N14d.TimeInLowMinutes,
				TimeInLowRecords: dto.Periods.N14d.TimeInLowRecords,

				TimeInTargetPercent: dto.Periods.N14d.TimeInTargetPercent,
				TimeInTargetMinutes: dto.Periods.N14d.TimeInTargetMinutes,
				TimeInTargetRecords: dto.Periods.N14d.TimeInTargetRecords,

				TimeInHighPercent: dto.Periods.N14d.TimeInHighPercent,
				TimeInHighMinutes: dto.Periods.N14d.TimeInHighMinutes,
				TimeInHighRecords: dto.Periods.N14d.TimeInHighRecords,

				TimeInVeryHighPercent: dto.Periods.N14d.TimeInVeryHighPercent,
				TimeInVeryHighMinutes: dto.Periods.N14d.TimeInVeryHighMinutes,
				TimeInVeryHighRecords: dto.Periods.N14d.TimeInVeryHighRecords,

				GlucoseManagementIndicator:    dto.Periods.N14d.GlucoseManagementIndicator,
				HasGlucoseManagementIndicator: dto.Periods.N14d.HasGlucoseManagementIndicator,
				AverageGlucose: &patients.AvgGlucose{
					Units: string(dto.Periods.N14d.AverageGlucose.Units),
					Value: float64(dto.Periods.N14d.AverageGlucose.Value),
				},
			}
		}

		if dto.Periods.N30d != nil {
			patientSummary.Periods["30d"] = &patients.Period{
				TimeCGMUsePercent:    dto.Periods.N30d.TimeCGMUsePercent,
				HasTimeCGMUsePercent: dto.Periods.N30d.HasTimeCGMUsePercent,
				TimeCGMUseMinutes:    dto.Periods.N30d.TimeCGMUseMinutes,
				TimeCGMUseRecords:    dto.Periods.N30d.TimeCGMUseRecords,

				TimeInVeryLowPercent: dto.Periods.N30d.TimeInVeryLowPercent,
				TimeInVeryLowMinutes: dto.Periods.N30d.TimeInVeryLowMinutes,
				TimeInVeryLowRecords: dto.Periods.N30d.TimeInVeryLowRecords,

				TimeInLowPercent: dto.Periods.N30d.TimeInLowPercent,
				TimeInLowMinutes: dto.Periods.N30d.TimeInLowMinutes,
				TimeInLowRecords: dto.Periods.N30d.TimeInLowRecords,

				TimeInTargetPercent: dto.Periods.N30d.TimeInTargetPercent,
				TimeInTargetMinutes: dto.Periods.N30d.TimeInTargetMinutes,
				TimeInTargetRecords: dto.Periods.N30d.TimeInTargetRecords,

				TimeInHighPercent: dto.Periods.N30d.TimeInHighPercent,
				TimeInHighMinutes: dto.Periods.N30d.TimeInHighMinutes,
				TimeInHighRecords: dto.Periods.N30d.TimeInHighRecords,

				TimeInVeryHighPercent: dto.Periods.N30d.TimeInVeryHighPercent,
				TimeInVeryHighMinutes: dto.Periods.N30d.TimeInVeryHighMinutes,
				TimeInVeryHighRecords: dto.Periods.N30d.TimeInVeryHighRecords,

				GlucoseManagementIndicator:    dto.Periods.N30d.GlucoseManagementIndicator,
				HasGlucoseManagementIndicator: dto.Periods.N30d.HasGlucoseManagementIndicator,
				AverageGlucose: &patients.AvgGlucose{
					Units: string(dto.Periods.N30d.AverageGlucose.Units),
					Value: float64(dto.Periods.N30d.AverageGlucose.Value),
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

		if _, exists := summary.Periods["1d"]; exists {
			patientSummary.Periods.N1d = &PatientSummaryPeriod{
				AverageGlucose: &AverageGlucose{
					Units: AverageGlucoseUnits(summary.Periods["1d"].AverageGlucose.Units),
					Value: float32(summary.Periods["1d"].AverageGlucose.Value),
				},
				GlucoseManagementIndicator:    summary.Periods["1d"].GlucoseManagementIndicator,
				HasGlucoseManagementIndicator: summary.Periods["1d"].HasGlucoseManagementIndicator,
				TimeCGMUseMinutes:             summary.Periods["1d"].TimeCGMUseMinutes,
				TimeCGMUsePercent:             summary.Periods["1d"].TimeCGMUsePercent,
				HasTimeCGMUsePercent:          summary.Periods["1d"].HasTimeCGMUsePercent,
				TimeCGMUseRecords:             summary.Periods["1d"].TimeCGMUseRecords,
				TimeInHighMinutes:             summary.Periods["1d"].TimeInHighMinutes,
				TimeInHighPercent:             summary.Periods["1d"].TimeInHighPercent,
				TimeInHighRecords:             summary.Periods["1d"].TimeInHighRecords,
				TimeInLowMinutes:              summary.Periods["1d"].TimeInLowMinutes,
				TimeInLowPercent:              summary.Periods["1d"].TimeInLowPercent,
				TimeInLowRecords:              summary.Periods["1d"].TimeInLowRecords,
				TimeInTargetMinutes:           summary.Periods["1d"].TimeInTargetMinutes,
				TimeInTargetPercent:           summary.Periods["1d"].TimeInTargetPercent,
				TimeInTargetRecords:           summary.Periods["1d"].TimeInTargetRecords,
				TimeInVeryHighMinutes:         summary.Periods["1d"].TimeInVeryHighMinutes,
				TimeInVeryHighPercent:         summary.Periods["1d"].TimeInVeryHighPercent,
				TimeInVeryHighRecords:         summary.Periods["1d"].TimeInVeryHighRecords,
				TimeInVeryLowMinutes:          summary.Periods["1d"].TimeInVeryLowMinutes,
				TimeInVeryLowPercent:          summary.Periods["1d"].TimeInVeryLowPercent,
				TimeInVeryLowRecords:          summary.Periods["1d"].TimeInVeryLowRecords,
			}
		}

		if _, exists := summary.Periods["7d"]; exists {
			patientSummary.Periods.N7d = &PatientSummaryPeriod{
				AverageGlucose: &AverageGlucose{
					Units: AverageGlucoseUnits(summary.Periods["7d"].AverageGlucose.Units),
					Value: float32(summary.Periods["7d"].AverageGlucose.Value),
				},
				GlucoseManagementIndicator:    summary.Periods["7d"].GlucoseManagementIndicator,
				HasGlucoseManagementIndicator: summary.Periods["7d"].HasGlucoseManagementIndicator,
				TimeCGMUseMinutes:             summary.Periods["7d"].TimeCGMUseMinutes,
				TimeCGMUsePercent:             summary.Periods["7d"].TimeCGMUsePercent,
				HasTimeCGMUsePercent:          summary.Periods["7d"].HasTimeCGMUsePercent,
				TimeCGMUseRecords:             summary.Periods["7d"].TimeCGMUseRecords,
				TimeInHighMinutes:             summary.Periods["7d"].TimeInHighMinutes,
				TimeInHighPercent:             summary.Periods["7d"].TimeInHighPercent,
				TimeInHighRecords:             summary.Periods["7d"].TimeInHighRecords,
				TimeInLowMinutes:              summary.Periods["7d"].TimeInLowMinutes,
				TimeInLowPercent:              summary.Periods["7d"].TimeInLowPercent,
				TimeInLowRecords:              summary.Periods["7d"].TimeInLowRecords,
				TimeInTargetMinutes:           summary.Periods["7d"].TimeInTargetMinutes,
				TimeInTargetPercent:           summary.Periods["7d"].TimeInTargetPercent,
				TimeInTargetRecords:           summary.Periods["7d"].TimeInTargetRecords,
				TimeInVeryHighMinutes:         summary.Periods["7d"].TimeInVeryHighMinutes,
				TimeInVeryHighPercent:         summary.Periods["7d"].TimeInVeryHighPercent,
				TimeInVeryHighRecords:         summary.Periods["7d"].TimeInVeryHighRecords,
				TimeInVeryLowMinutes:          summary.Periods["7d"].TimeInVeryLowMinutes,
				TimeInVeryLowPercent:          summary.Periods["7d"].TimeInVeryLowPercent,
				TimeInVeryLowRecords:          summary.Periods["7d"].TimeInVeryLowRecords,
			}
		}

		if _, exists := summary.Periods["14d"]; exists {
			patientSummary.Periods.N14d = &PatientSummaryPeriod{
				AverageGlucose: &AverageGlucose{
					Units: AverageGlucoseUnits(summary.Periods["14d"].AverageGlucose.Units),
					Value: float32(summary.Periods["14d"].AverageGlucose.Value),
				},
				GlucoseManagementIndicator:    summary.Periods["14d"].GlucoseManagementIndicator,
				HasGlucoseManagementIndicator: summary.Periods["14d"].HasGlucoseManagementIndicator,
				TimeCGMUseMinutes:             summary.Periods["14d"].TimeCGMUseMinutes,
				TimeCGMUsePercent:             summary.Periods["14d"].TimeCGMUsePercent,
				HasTimeCGMUsePercent:          summary.Periods["14d"].HasTimeCGMUsePercent,
				TimeCGMUseRecords:             summary.Periods["14d"].TimeCGMUseRecords,
				TimeInHighMinutes:             summary.Periods["14d"].TimeInHighMinutes,
				TimeInHighPercent:             summary.Periods["14d"].TimeInHighPercent,
				TimeInHighRecords:             summary.Periods["14d"].TimeInHighRecords,
				TimeInLowMinutes:              summary.Periods["14d"].TimeInLowMinutes,
				TimeInLowPercent:              summary.Periods["14d"].TimeInLowPercent,
				TimeInLowRecords:              summary.Periods["14d"].TimeInLowRecords,
				TimeInTargetMinutes:           summary.Periods["14d"].TimeInTargetMinutes,
				TimeInTargetPercent:           summary.Periods["14d"].TimeInTargetPercent,
				TimeInTargetRecords:           summary.Periods["14d"].TimeInTargetRecords,
				TimeInVeryHighMinutes:         summary.Periods["14d"].TimeInVeryHighMinutes,
				TimeInVeryHighPercent:         summary.Periods["14d"].TimeInVeryHighPercent,
				TimeInVeryHighRecords:         summary.Periods["14d"].TimeInVeryHighRecords,
				TimeInVeryLowMinutes:          summary.Periods["14d"].TimeInVeryLowMinutes,
				TimeInVeryLowPercent:          summary.Periods["14d"].TimeInVeryLowPercent,
				TimeInVeryLowRecords:          summary.Periods["14d"].TimeInVeryLowRecords,
			}
		}

		if _, exists := summary.Periods["30d"]; exists {
			patientSummary.Periods.N30d = &PatientSummaryPeriod{
				AverageGlucose: &AverageGlucose{
					Units: AverageGlucoseUnits(summary.Periods["30d"].AverageGlucose.Units),
					Value: float32(summary.Periods["30d"].AverageGlucose.Value),
				},
				GlucoseManagementIndicator: summary.Periods["30d"].GlucoseManagementIndicator,
				TimeCGMUseMinutes:          summary.Periods["30d"].TimeCGMUseMinutes,
				TimeCGMUsePercent:          summary.Periods["30d"].TimeCGMUsePercent,
				TimeCGMUseRecords:          summary.Periods["30d"].TimeCGMUseRecords,
				TimeInHighMinutes:          summary.Periods["30d"].TimeInHighMinutes,
				TimeInHighPercent:          summary.Periods["30d"].TimeInHighPercent,
				TimeInHighRecords:          summary.Periods["30d"].TimeInHighRecords,
				TimeInLowMinutes:           summary.Periods["30d"].TimeInLowMinutes,
				TimeInLowPercent:           summary.Periods["30d"].TimeInLowPercent,
				TimeInLowRecords:           summary.Periods["30d"].TimeInLowRecords,
				TimeInTargetMinutes:        summary.Periods["30d"].TimeInTargetMinutes,
				TimeInTargetPercent:        summary.Periods["30d"].TimeInTargetPercent,
				TimeInTargetRecords:        summary.Periods["30d"].TimeInTargetRecords,
				TimeInVeryHighMinutes:      summary.Periods["30d"].TimeInVeryHighMinutes,
				TimeInVeryHighPercent:      summary.Periods["30d"].TimeInVeryHighPercent,
				TimeInVeryHighRecords:      summary.Periods["30d"].TimeInVeryHighRecords,
				TimeInVeryLowMinutes:       summary.Periods["30d"].TimeInVeryLowMinutes,
				TimeInVeryLowPercent:       summary.Periods["30d"].TimeInVeryLowPercent,
				TimeInVeryLowRecords:       summary.Periods["30d"].TimeInVeryLowRecords,
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

	fmt.Println("sorts generated:")
	for _, s := range sorts {
		fmt.Println(s)
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
