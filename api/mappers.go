package api

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/migration"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	id := Id(c.Id.Hex())
	canMigrate := c.CanMigrate()

	dto := Clinic{
		Id:                      &id,
		Name:                    pstr(c.Name),
		ShareCode:               c.CanonicalShareCode,
		CanMigrate:              &canMigrate,
		ClinicType:              stringToClinicType(c.ClinicType),
		ClinicSize:              stringToClinicSize(c.ClinicSize),
		Address:                 c.Address,
		City:                    c.City,
		PostalCode:              c.PostalCode,
		State:                   c.State,
		Country:                 c.Country,
		Website:                 c.Website,
		CreatedTime:             &c.CreatedTime,
		UpdatedTime:             &c.UpdatedTime,
		Tier:                    &tier,
		TierDescription:         strp(clinics.GetTierDescription(tier)),
		PreferredBgUnits:        units,
		SuppressedNotifications: (*SuppressedNotifications)(&c.SuppressedNotifications),
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
	if c.PatientTags != nil {
		var patientTags []PatientTag
		for _, n := range c.PatientTags {
			patientTags = append(patientTags, PatientTag{
				Id:   strp(n.Id.Hex()),
				Name: n.Name,
			})
		}
		dto.PatientTags = &patientTags
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
		CreatedTime: &clinician.CreatedTime,
		UpdatedTime: &clinician.UpdatedTime,
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
		Id:            strpuseridp(patient.UserId),
		Mrn:           patient.Mrn,
		Permissions:   NewPermissionsDto(patient.Permissions),
		Tags:          NewPatientTagsDto(patient.Tags),
		DataSources:   NewPatientDataSourcesDto(patient.DataSources),
		TargetDevices: patient.TargetDevices,
		CreatedTime:   &patient.CreatedTime,
		UpdatedTime:   &patient.UpdatedTime,
		Summary:       NewSummaryDto(patient.Summary),
	}
	if patient.BirthDate != nil && strtodatep(patient.BirthDate) != nil {
		dto.BirthDate = *strtodatep(patient.BirthDate)
	}
	if !patient.LastUploadReminderTime.IsZero() {
		dto.LastUploadReminderTime = &patient.LastUploadReminderTime
	}
	if !patient.LastRequestedDexcomConnectTime.IsZero() {
		dto.LastRequestedDexcomConnectTime = &patient.LastRequestedDexcomConnectTime
	}
	return dto
}

func NewPatient(dto Patient) patients.Patient {
	patient := patients.Patient{
		Email:         pstrToLower(dto.Email),
		BirthDate:     strp(dto.BirthDate.Format(dateFormat)),
		FullName:      &dto.FullName,
		Mrn:           dto.Mrn,
		TargetDevices: dto.TargetDevices,
	}

	if dto.Tags != nil {
		tags := store.ObjectIDSFromStringArray(*dto.Tags)
		patient.Tags = &tags
	}

	if dto.DataSources != nil {
		var dataSources []patients.DataSource
		for _, d := range *dto.DataSources {

			newDataSource := patients.DataSource{
				ProviderName: string(d.ProviderName),
				State:        string(d.State),
			}

			if d.DataSourceId != nil {
				dataSourceObjectId, _ := primitive.ObjectIDFromHex(*d.DataSourceId)
				newDataSource.DataSourceId = &dataSourceObjectId
			}

			if d.ModifiedTime != nil {
				modifiedTime, _ := time.Parse(time.RFC3339Nano, string(*d.ModifiedTime))
				newDataSource.ModifiedTime = &modifiedTime
			}

			if d.ExpirationTime != nil {
				expirationTime, _ := time.Parse(time.RFC3339Nano, string(*d.ExpirationTime))
				newDataSource.ExpirationTime = &expirationTime
			}

			dataSources = append(dataSources, newDataSource)
		}
		patient.DataSources = &dataSources
	}

	return patient
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
		}

		if dto.CgmStats.Dates != nil {
			patientSummary.CGM.Dates = &patients.Dates{
				LastUpdatedDate: dto.CgmStats.Dates.LastUpdatedDate,

				HasLastUploadDate: dto.CgmStats.Dates.HasLastUploadDate,
				LastUploadDate:    dto.CgmStats.Dates.LastUploadDate,

				HasOutdatedSince: dto.CgmStats.Dates.HasOutdatedSince,
				OutdatedSince:    dto.CgmStats.Dates.OutdatedSince,

				HasFirstData: dto.CgmStats.Dates.HasFirstData,
				FirstData:    dto.CgmStats.Dates.FirstData,

				HasLastData: dto.CgmStats.Dates.HasLastData,
				LastData:    dto.CgmStats.Dates.LastData,
			}
		}

		if dto.CgmStats.Config != nil {
			patientSummary.CGM.Config = &patients.Config{
				SchemaVersion:            dto.CgmStats.Config.SchemaVersion,
				HighGlucoseThreshold:     dto.CgmStats.Config.HighGlucoseThreshold,
				VeryHighGlucoseThreshold: dto.CgmStats.Config.VeryLowGlucoseThreshold,
				LowGlucoseThreshold:      dto.CgmStats.Config.LowGlucoseThreshold,
				VeryLowGlucoseThreshold:  dto.CgmStats.Config.VeryLowGlucoseThreshold,
			}
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
					HasTimeCGMUsePercent: sourcePeriods[i].HasTimeCGMUsePercent,
					TimeCGMUsePercent:    sourcePeriods[i].TimeCGMUsePercent,

					HasTimeCGMUseMinutes: sourcePeriods[i].HasTimeCGMUseMinutes,
					TimeCGMUseMinutes:    sourcePeriods[i].TimeCGMUseMinutes,

					HasTimeCGMUseRecords: sourcePeriods[i].HasTimeCGMUseRecords,
					TimeCGMUseRecords:    sourcePeriods[i].TimeCGMUseRecords,

					HasTimeInVeryLowPercent: sourcePeriods[i].HasTimeInVeryLowPercent,
					TimeInVeryLowPercent:    sourcePeriods[i].TimeInVeryLowPercent,

					HasTimeInVeryLowMinutes: sourcePeriods[i].HasTimeInVeryLowMinutes,
					TimeInVeryLowMinutes:    sourcePeriods[i].TimeInVeryLowMinutes,

					HasTimeInVeryLowRecords: sourcePeriods[i].HasTimeInVeryLowRecords,
					TimeInVeryLowRecords:    sourcePeriods[i].TimeInVeryLowRecords,

					HasTimeInLowPercent: sourcePeriods[i].HasTimeInLowPercent,
					TimeInLowPercent:    sourcePeriods[i].TimeInLowPercent,

					HasTimeInLowMinutes: sourcePeriods[i].HasTimeInLowMinutes,
					TimeInLowMinutes:    sourcePeriods[i].TimeInLowMinutes,

					HasTimeInLowRecords: sourcePeriods[i].HasTimeInLowRecords,
					TimeInLowRecords:    sourcePeriods[i].TimeInLowRecords,

					HasTimeInTargetPercent: sourcePeriods[i].HasTimeInTargetPercent,
					TimeInTargetPercent:    sourcePeriods[i].TimeInTargetPercent,

					HasTimeInTargetMinutes: sourcePeriods[i].HasTimeInTargetMinutes,
					TimeInTargetMinutes:    sourcePeriods[i].TimeInTargetMinutes,

					HasTimeInTargetRecords: sourcePeriods[i].HasTimeInTargetRecords,
					TimeInTargetRecords:    sourcePeriods[i].TimeInTargetRecords,

					HasTimeInHighPercent: sourcePeriods[i].HasTimeInHighPercent,
					TimeInHighPercent:    sourcePeriods[i].TimeInHighPercent,

					HasTimeInHighMinutes: sourcePeriods[i].HasTimeInHighMinutes,
					TimeInHighMinutes:    sourcePeriods[i].TimeInHighMinutes,

					HasTimeInHighRecords: sourcePeriods[i].HasTimeInHighRecords,
					TimeInHighRecords:    sourcePeriods[i].TimeInHighRecords,

					HasTimeInVeryHighPercent: sourcePeriods[i].HasTimeInVeryHighPercent,
					TimeInVeryHighPercent:    sourcePeriods[i].TimeInVeryHighPercent,

					HasTimeInVeryHighMinutes: sourcePeriods[i].HasTimeInVeryHighMinutes,
					TimeInVeryHighMinutes:    sourcePeriods[i].TimeInVeryHighMinutes,

					HasTimeInVeryHighRecords: sourcePeriods[i].HasTimeInVeryHighRecords,
					TimeInVeryHighRecords:    sourcePeriods[i].TimeInVeryHighRecords,

					HasGlucoseManagementIndicator: sourcePeriods[i].HasGlucoseManagementIndicator,
					GlucoseManagementIndicator:    sourcePeriods[i].GlucoseManagementIndicator,

					HasAverageGlucose: sourcePeriods[i].HasAverageGlucose,
					AverageGlucose:    averageGlucose,

					HasTotalRecords: sourcePeriods[i].HasTotalRecords,
					TotalRecords:    sourcePeriods[i].TotalRecords,

					HasAverageDailyRecords: sourcePeriods[i].HasAverageDailyRecords,
					AverageDailyRecords:    sourcePeriods[i].AverageDailyRecords,
				}
			}
		}
	}

	if dto.BgmStats != nil {
		patientSummary.BGM = &patients.BGMStats{
			Periods:    make(map[string]*patients.BGMPeriod),
			TotalHours: dto.BgmStats.TotalHours,
		}

		if dto.BgmStats.Config != nil {
			patientSummary.BGM.Config = &patients.Config{
				SchemaVersion:            dto.BgmStats.Config.SchemaVersion,
				HighGlucoseThreshold:     dto.BgmStats.Config.HighGlucoseThreshold,
				VeryHighGlucoseThreshold: dto.BgmStats.Config.VeryLowGlucoseThreshold,
				LowGlucoseThreshold:      dto.BgmStats.Config.LowGlucoseThreshold,
				VeryLowGlucoseThreshold:  dto.BgmStats.Config.VeryLowGlucoseThreshold,
			}
		}
		if dto.BgmStats.Dates != nil {
			patientSummary.BGM.Dates = &patients.Dates{
				LastUpdatedDate: dto.BgmStats.Dates.LastUpdatedDate,

				HasLastUploadDate: dto.BgmStats.Dates.HasLastUploadDate,
				LastUploadDate:    dto.BgmStats.Dates.LastUploadDate,

				HasOutdatedSince: dto.BgmStats.Dates.HasOutdatedSince,
				OutdatedSince:    dto.BgmStats.Dates.OutdatedSince,

				HasFirstData: dto.BgmStats.Dates.HasFirstData,
				FirstData:    dto.BgmStats.Dates.FirstData,

				HasLastData: dto.BgmStats.Dates.HasLastData,
				LastData:    dto.BgmStats.Dates.LastData,
			}
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

				patientSummary.BGM.Periods[i] = &patients.BGMPeriod{
					HasTimeInVeryLowPercent: sourcePeriods[i].HasTimeInVeryLowPercent,
					TimeInVeryLowPercent:    sourcePeriods[i].TimeInVeryLowPercent,

					HasTimeInVeryLowRecords: sourcePeriods[i].HasTimeInVeryLowRecords,
					TimeInVeryLowRecords:    sourcePeriods[i].TimeInVeryLowRecords,

					HasTimeInLowPercent: sourcePeriods[i].HasTimeInLowPercent,
					TimeInLowPercent:    sourcePeriods[i].TimeInLowPercent,

					HasTimeInLowRecords: sourcePeriods[i].HasTimeInLowRecords,
					TimeInLowRecords:    sourcePeriods[i].TimeInLowRecords,

					HasTimeInTargetPercent: sourcePeriods[i].HasTimeInTargetPercent,
					TimeInTargetPercent:    sourcePeriods[i].TimeInTargetPercent,

					HasTimeInTargetRecords: sourcePeriods[i].HasTimeInTargetRecords,
					TimeInTargetRecords:    sourcePeriods[i].TimeInTargetRecords,

					HasTimeInHighPercent: sourcePeriods[i].HasTimeInHighPercent,
					TimeInHighPercent:    sourcePeriods[i].TimeInHighPercent,

					HasTimeInHighRecords: sourcePeriods[i].HasTimeInHighRecords,
					TimeInHighRecords:    sourcePeriods[i].TimeInHighRecords,

					HasTimeInVeryHighPercent: sourcePeriods[i].HasTimeInVeryHighPercent,
					TimeInVeryHighPercent:    sourcePeriods[i].TimeInVeryHighPercent,

					HasTimeInVeryHighRecords: sourcePeriods[i].HasTimeInVeryHighRecords,
					TimeInVeryHighRecords:    sourcePeriods[i].TimeInVeryHighRecords,

					HasAverageGlucose: sourcePeriods[i].HasAverageGlucose,
					AverageGlucose:    averageGlucose,

					HasTotalRecords: sourcePeriods[i].HasTotalRecords,
					TotalRecords:    sourcePeriods[i].TotalRecords,

					HasAverageDailyRecords: sourcePeriods[i].HasAverageDailyRecords,
					AverageDailyRecords:    sourcePeriods[i].AverageDailyRecords,
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
			Periods:    &PatientCGMPeriods{},
			TotalHours: summary.CGM.TotalHours,
		}

		if summary.CGM.Config != nil {
			patientSummary.CgmStats.Config = &PatientSummaryConfig{
				SchemaVersion:            summary.CGM.Config.SchemaVersion,
				HighGlucoseThreshold:     summary.CGM.Config.HighGlucoseThreshold,
				VeryHighGlucoseThreshold: summary.CGM.Config.VeryHighGlucoseThreshold,
				LowGlucoseThreshold:      summary.CGM.Config.LowGlucoseThreshold,
				VeryLowGlucoseThreshold:  summary.CGM.Config.VeryLowGlucoseThreshold,
			}
		}

		if summary.CGM.Dates != nil {
			patientSummary.CgmStats.Dates = &PatientSummaryDates{
				LastUpdatedDate: summary.CGM.Dates.LastUpdatedDate,

				HasFirstData: summary.CGM.Dates.HasFirstData,
				FirstData:    summary.CGM.Dates.FirstData,

				HasLastUploadDate: summary.CGM.Dates.HasLastUploadDate,
				LastUploadDate:    summary.CGM.Dates.LastUploadDate,

				HasLastData: summary.CGM.Dates.HasLastData,
				LastData:    summary.CGM.Dates.LastData,

				HasOutdatedSince: summary.CGM.Dates.HasOutdatedSince,
				OutdatedSince:    summary.CGM.Dates.OutdatedSince,
			}
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

				destPeriods[i].HasAverageDailyRecords = summary.CGM.Periods[i].HasAverageDailyRecords
				destPeriods[i].AverageDailyRecords = summary.CGM.Periods[i].AverageDailyRecords

				destPeriods[i].HasTotalRecords = summary.CGM.Periods[i].HasTotalRecords
				destPeriods[i].TotalRecords = summary.CGM.Periods[i].TotalRecords

				destPeriods[i].HasTimeCGMUsePercent = summary.CGM.Periods[i].HasTimeCGMUsePercent
				destPeriods[i].TimeCGMUsePercent = summary.CGM.Periods[i].TimeCGMUsePercent

				destPeriods[i].HasTimeCGMUseMinutes = summary.CGM.Periods[i].HasTimeCGMUseMinutes
				destPeriods[i].TimeCGMUseMinutes = summary.CGM.Periods[i].TimeCGMUseMinutes

				destPeriods[i].HasTimeCGMUseRecords = summary.CGM.Periods[i].HasTimeCGMUseRecords
				destPeriods[i].TimeCGMUseRecords = summary.CGM.Periods[i].TimeCGMUseRecords

				destPeriods[i].HasTimeInHighPercent = summary.CGM.Periods[i].HasTimeInHighPercent
				destPeriods[i].TimeInHighPercent = summary.CGM.Periods[i].TimeInHighPercent

				destPeriods[i].HasTimeInHighMinutes = summary.CGM.Periods[i].HasTimeInHighMinutes
				destPeriods[i].TimeInHighMinutes = summary.CGM.Periods[i].TimeInHighMinutes

				destPeriods[i].HasTimeInHighRecords = summary.CGM.Periods[i].HasTimeInHighRecords
				destPeriods[i].TimeInHighRecords = summary.CGM.Periods[i].TimeInHighRecords

				destPeriods[i].HasTimeInLowPercent = summary.CGM.Periods[i].HasTimeInLowPercent
				destPeriods[i].TimeInLowPercent = summary.CGM.Periods[i].TimeInLowPercent

				destPeriods[i].HasTimeInLowMinutes = summary.CGM.Periods[i].HasTimeInLowMinutes
				destPeriods[i].TimeInLowMinutes = summary.CGM.Periods[i].TimeInLowMinutes

				destPeriods[i].HasTimeInLowRecords = summary.CGM.Periods[i].HasTimeInLowRecords
				destPeriods[i].TimeInLowRecords = summary.CGM.Periods[i].TimeInLowRecords

				destPeriods[i].HasTimeInTargetPercent = summary.CGM.Periods[i].HasTimeInTargetPercent
				destPeriods[i].TimeInTargetPercent = summary.CGM.Periods[i].TimeInTargetPercent

				destPeriods[i].HasTimeInTargetRecords = summary.CGM.Periods[i].HasTimeInTargetRecords
				destPeriods[i].TimeInTargetRecords = summary.CGM.Periods[i].TimeInTargetRecords

				destPeriods[i].HasTimeInTargetMinutes = summary.CGM.Periods[i].HasTimeInTargetMinutes
				destPeriods[i].TimeInTargetMinutes = summary.CGM.Periods[i].TimeInTargetMinutes

				destPeriods[i].HasTimeInVeryHighPercent = summary.CGM.Periods[i].HasTimeInVeryHighPercent
				destPeriods[i].TimeInVeryHighPercent = summary.CGM.Periods[i].TimeInVeryHighPercent

				destPeriods[i].HasTimeInVeryHighMinutes = summary.CGM.Periods[i].HasTimeInVeryHighMinutes
				destPeriods[i].TimeInVeryHighMinutes = summary.CGM.Periods[i].TimeInVeryHighMinutes

				destPeriods[i].HasTimeInVeryHighRecords = summary.CGM.Periods[i].HasTimeInVeryHighRecords
				destPeriods[i].TimeInVeryHighRecords = summary.CGM.Periods[i].TimeInVeryHighRecords

				destPeriods[i].HasTimeInVeryLowPercent = summary.CGM.Periods[i].HasTimeInVeryLowPercent
				destPeriods[i].TimeInVeryLowPercent = summary.CGM.Periods[i].TimeInVeryLowPercent

				destPeriods[i].HasTimeInVeryLowMinutes = summary.CGM.Periods[i].HasTimeInVeryLowMinutes
				destPeriods[i].TimeInVeryLowMinutes = summary.CGM.Periods[i].TimeInVeryLowMinutes

				destPeriods[i].HasTimeInVeryLowRecords = summary.CGM.Periods[i].HasTimeInVeryLowRecords
				destPeriods[i].TimeInVeryLowRecords = summary.CGM.Periods[i].TimeInVeryLowRecords
			}
		}
	}

	if summary.BGM != nil {
		patientSummary.BgmStats = &PatientBGMStats{
			Periods:    &PatientBGMPeriods{},
			TotalHours: summary.BGM.TotalHours,
		}

		if summary.BGM.Config != nil {
			patientSummary.BgmStats.Config = &PatientSummaryConfig{
				SchemaVersion:            summary.BGM.Config.SchemaVersion,
				HighGlucoseThreshold:     summary.BGM.Config.HighGlucoseThreshold,
				VeryHighGlucoseThreshold: summary.BGM.Config.VeryHighGlucoseThreshold,
				LowGlucoseThreshold:      summary.BGM.Config.LowGlucoseThreshold,
				VeryLowGlucoseThreshold:  summary.BGM.Config.VeryLowGlucoseThreshold,
			}
		}
		if summary.BGM.Dates != nil {
			patientSummary.BgmStats.Dates = &PatientSummaryDates{
				LastUpdatedDate: summary.BGM.Dates.LastUpdatedDate,

				HasFirstData: summary.BGM.Dates.HasFirstData,
				FirstData:    summary.BGM.Dates.FirstData,

				HasLastUploadDate: summary.BGM.Dates.HasLastUploadDate,
				LastUploadDate:    summary.BGM.Dates.LastUploadDate,

				HasLastData: summary.BGM.Dates.HasLastData,
				LastData:    summary.BGM.Dates.LastData,

				HasOutdatedSince: summary.BGM.Dates.HasOutdatedSince,
				OutdatedSince:    summary.BGM.Dates.OutdatedSince,
			}
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

				destPeriods[i].HasAverageDailyRecords = summary.BGM.Periods[i].HasAverageDailyRecords
				destPeriods[i].AverageDailyRecords = summary.BGM.Periods[i].AverageDailyRecords

				destPeriods[i].HasTotalRecords = summary.BGM.Periods[i].HasTotalRecords
				destPeriods[i].TotalRecords = summary.BGM.Periods[i].TotalRecords

				destPeriods[i].HasTimeInHighPercent = summary.BGM.Periods[i].HasTimeInHighPercent
				destPeriods[i].TimeInHighPercent = summary.BGM.Periods[i].TimeInHighPercent

				destPeriods[i].HasTimeInHighRecords = summary.BGM.Periods[i].HasTimeInHighRecords
				destPeriods[i].TimeInHighRecords = summary.BGM.Periods[i].TimeInHighRecords

				destPeriods[i].HasTimeInLowPercent = summary.BGM.Periods[i].HasTimeInLowPercent
				destPeriods[i].TimeInLowPercent = summary.BGM.Periods[i].TimeInLowPercent

				destPeriods[i].HasTimeInLowRecords = summary.BGM.Periods[i].HasTimeInLowRecords
				destPeriods[i].TimeInLowRecords = summary.BGM.Periods[i].TimeInLowRecords

				destPeriods[i].HasTimeInTargetPercent = summary.BGM.Periods[i].HasTimeInTargetPercent
				destPeriods[i].TimeInTargetPercent = summary.BGM.Periods[i].TimeInTargetPercent

				destPeriods[i].HasTimeInTargetRecords = summary.BGM.Periods[i].HasTimeInTargetRecords
				destPeriods[i].TimeInTargetRecords = summary.BGM.Periods[i].TimeInTargetRecords

				destPeriods[i].HasTimeInVeryHighPercent = summary.BGM.Periods[i].HasTimeInVeryHighPercent
				destPeriods[i].TimeInVeryHighPercent = summary.BGM.Periods[i].TimeInVeryHighPercent

				destPeriods[i].HasTimeInVeryHighRecords = summary.BGM.Periods[i].HasTimeInVeryHighRecords
				destPeriods[i].TimeInVeryHighRecords = summary.BGM.Periods[i].TimeInVeryHighRecords

				destPeriods[i].HasTimeInVeryLowPercent = summary.BGM.Periods[i].HasTimeInVeryLowPercent
				destPeriods[i].TimeInVeryLowPercent = summary.BGM.Periods[i].TimeInVeryLowPercent

				destPeriods[i].HasTimeInVeryLowRecords = summary.BGM.Periods[i].HasTimeInVeryLowRecords
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

func NewPatientTagsDto(tags *[]primitive.ObjectID) *[]string {
	var tagIds []string
	if tags != nil {
		for _, id := range *tags {
			tagIds = append(tagIds, id.Hex())
		}
	}
	return &tagIds
}

func NewPatientDataSourcesDto(dataSources *[]patients.DataSource) *[]DataSource {
	if dataSources == nil {
		return nil
	}

	dtos := make([]DataSource, 0)

	if dataSources != nil {
		for _, d := range *dataSources {
			newDataSource := DataSource{
				ProviderName: d.ProviderName,
				State:        DataSourceState(d.State),
			}

			if d.DataSourceId != nil {
				dataSourceId := d.DataSourceId.Hex()
				newDataSource.DataSourceId = &dataSourceId
			}

			if d.ModifiedTime != nil {
				modifiedTime := DateTime(d.ModifiedTime.Format(time.RFC3339Nano))
				newDataSource.ModifiedTime = &modifiedTime
			}

			if d.ExpirationTime != nil {
				expirationTime := DateTime(d.ExpirationTime.Format(time.RFC3339Nano))
				newDataSource.ExpirationTime = &expirationTime
			}

			dtos = append(dtos, newDataSource)
		}
	}

	return &dtos
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
		CreatedTime: &migration.CreatedTime,
		UpdatedTime: &migration.UpdatedTime,
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

func NewMembershipRestrictionsDto(restrictions []clinics.MembershipRestrictions) MembershipRestrictions {
	dto := MembershipRestrictions{}
	var dtos []MembershipRestriction
	for _, r := range restrictions {
		restriction := MembershipRestriction{}
		restriction.EmailDomain = r.EmailDomain
		if r.RequiredIdp != "" {
			restriction.RequiredIdp = strp(r.RequiredIdp)
		}
		dtos = append(dtos, restriction)
	}
	if len(dtos) > 0 {
		dto.Restrictions = &dtos
	}

	return dto
}

func NewMembershipRestrictions(dto MembershipRestrictions) []clinics.MembershipRestrictions {
	var restrictions []clinics.MembershipRestrictions
	if dto.Restrictions != nil {
		for _, r := range *dto.Restrictions {
			restriction := clinics.MembershipRestrictions{
				EmailDomain: r.EmailDomain,
				RequiredIdp: pstr(r.RequiredIdp),
			}
			restrictions = append(restrictions, restriction)
		}
	}

	return restrictions
}

func ParseSort(sort *Sort, typ *string, period *string) ([]*store.Sort, error) {
	if sort == nil {
		return nil, nil
	}

	if typ == nil {
		return nil, fmt.Errorf("%w: invalid sort parameter, missing type", errors.BadRequest)
	} else if *typ != "cgm" && *typ != "bgm" {
		return nil, fmt.Errorf("%w: invalid sort parameter, invalid type", errors.BadRequest)
	}

	if period == nil {
		return nil, fmt.Errorf("%w: invalid sort parameter, missing period", errors.BadRequest)
	} else if *period != "1d" && *period != "7d" && *period != "14d" && *period != "30d" {
		return nil, fmt.Errorf("%w: invalid sort parameter, invalid period", errors.BadRequest)
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
	} else if !isSortAttributeValid(result.Attribute, *typ) {
		return nil, fmt.Errorf("%w: invalid sort parameter, invalid sort attribute", errors.BadRequest)
	}

	var expandedSorts = map[string]string{
		"lastUpdatedDate": "summary." + *typ + "Stats.dates.lastUpdatedDate",

		"hasLastUploadDate": "summary." + *typ + "Stats.dates.hasLastUploadDate",
		"lastUploadDate":    "summary." + *typ + "Stats.dates.lastUploadDate",

		"hasFirstData": "summary." + *typ + "Stats.dates.hasFirstData",
		"firstData":    "summary." + *typ + "Stats.dates.firstData",

		"hasLastData": "summary." + *typ + "Stats.dates.hasLastData",
		"lastData":    "summary." + *typ + "Stats.dates.lastData",

		"hasOutdatedSince": "summary." + *typ + "Stats.dates.hasOutdatedSince",
		"outdatedSince":    "summary." + *typ + "Stats.dates.outdatedSince",

		"hasAverageGlucose": "summary." + *typ + "Stats.periods." + *period + ".hasAverageGlucose",
		"averageGlucose":    "summary." + *typ + "Stats.periods." + *period + ".averageGlucose.value",

		"hasGlucoseManagementIndicator": "summary." + *typ + "Stats.periods." + *period + ".hasGlucoseManagementIndicator",
		"glucoseManagementIndicator":    "summary." + *typ + "Stats.periods." + *period + ".glucoseManagementIndicator",

		"hasTimeCGMUsePercent": "summary." + *typ + "Stats.periods." + *period + ".hasTimeCGMUsePercent",
		"timeCGMUsePercent":    "summary." + *typ + "Stats.periods." + *period + ".timeCGMUsePercent",

		"hasTimeCGMUseRecords": "summary." + *typ + "Stats.periods." + *period + ".hasTimeCGMUseRecords",
		"timeCGMUseRecords":    "summary." + *typ + "Stats.periods." + *period + ".timeCGMUseRecords",

		"hasTimeCGMUseMinutes": "summary." + *typ + "Stats.periods." + *period + ".hasTimeCGMUseMinutes",
		"timeCGMUseMinutes":    "summary." + *typ + "Stats.periods." + *period + ".timeCGMUseMinutes",

		"hasTimeInTargetPercent": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInTargetPercent",
		"timeInTargetPercent":    "summary." + *typ + "Stats.periods." + *period + ".timeInTargetPercent",

		"hasTimeInTargetRecords": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInTargetRecords",
		"timeInTargetRecords":    "summary." + *typ + "Stats.periods." + *period + ".timeInTargetRecords",

		"hasTimeInTargetMinutes": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInTargetMinutes",
		"timeInTargetMinutes":    "summary." + *typ + "Stats.periods." + *period + ".timeInTargetMinutes",

		"hasTimeInLowPercent": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInLowPercent",
		"timeInLowPercent":    "summary." + *typ + "Stats.periods." + *period + ".timeInLowPercent",

		"hasTimeInLowRecords": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInLowRecords",
		"timeInLowRecords":    "summary." + *typ + "Stats.periods." + *period + ".timeInLowRecords",

		"hasTimeInLowMinutes": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInLowMinutes",
		"timeInLowMinutes":    "summary." + *typ + "Stats.periods." + *period + ".timeInLowMinutes",

		"hasTimeInVeryLowPercent": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryLowPercent",
		"timeInVeryLowPercent":    "summary." + *typ + "Stats.periods." + *period + ".timeInVeryLowPercent",

		"hasTimeInVeryLowRecords": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryLowRecords",
		"timeInVeryLowRecords":    "summary." + *typ + "Stats.periods." + *period + ".timeInVeryLowRecords",

		"hasTimeInVeryLowMinutes": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryLowMinutes",
		"timeInVeryLowMinutes":    "summary." + *typ + "Stats.periods." + *period + ".timeInVeryLowMinutes",

		"hasTimeInHighPercent": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInHighPercent",
		"timeInHighPercent":    "summary." + *typ + "Stats.periods." + *period + ".timeInHighPercent",

		"hasTimeInHighMinutes": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInHighMinutes",
		"timeInHighMinutes":    "summary." + *typ + "Stats.periods." + *period + ".timeInHighMinutes",

		"hasTimeInHighRecords": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInHighRecords",
		"timeInHighRecords":    "summary." + *typ + "Stats.periods." + *period + ".timeInHighRecords",

		"hasTimeInVeryHighPercent": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryHighPercent",
		"timeInVeryHighPercent":    "summary." + *typ + "Stats.periods." + *period + ".timeInVeryHighPercent",

		"hasTimeInVeryHighRecords": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryHighRecords",
		"timeInVeryHighRecords":    "summary." + *typ + "Stats.periods." + *period + ".timeInVeryHighRecords",

		"hasTimeInVeryHighMinutes": "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryHighMinutes",
		"timeInVeryHighMinutes":    "summary." + *typ + "Stats.periods." + *period + ".timeInVeryHighMinutes",

		"hasAverageDailyRecords": "summary." + *typ + "Stats.periods." + *period + ".hasAverageDailyRecords",
		"averageDailyRecords":    "summary." + *typ + "Stats.periods." + *period + ".averageDailyRecords",

		"hasTotalRecords": "summary." + *typ + "Stats.periods." + *period + ".hasTotalRecords",
		"totalRecords":    "summary." + *typ + "Stats.periods." + *period + ".totalRecords",
	}

	var extraSort = map[string]string{
		expandedSorts["lastUploadDate"]: expandedSorts["hasLastUploadDate"],
		expandedSorts["lastData"]:       expandedSorts["hasLastData"],
		expandedSorts["firstData"]:      expandedSorts["hasFirstData"],
		expandedSorts["outdatedSince"]:  expandedSorts["hasOutdatedSince"],

		expandedSorts["glucoseManagementIndicator"]: expandedSorts["hasGlucoseManagementIndicator"],
		expandedSorts["averageGlucose"]:             expandedSorts["hasAverageGlucose"],
		expandedSorts["totalRecords"]:               expandedSorts["hasTotalRecords"],
		expandedSorts["averageDailyRecords"]:        expandedSorts["hasAverageDailyRecords"],

		expandedSorts["timeCGMUsePercent"]: expandedSorts["hasTimeCGMUsePercent"],
		expandedSorts["timeCGMUseRecords"]: expandedSorts["hasTimeCGMUseRecords"],
		expandedSorts["timeCGMUseMinutes"]: expandedSorts["hasTimeCGMUseMinutes"],

		expandedSorts["timeInTargetPercent"]: expandedSorts["hasTimeInTargetPercent"],
		expandedSorts["timeInTargetRecords"]: expandedSorts["hasTimeInTargetRecords"],
		expandedSorts["timeInTargetMinutes"]: expandedSorts["hasTimeInTargetMinutes"],

		expandedSorts["timeInLowPercent"]: expandedSorts["hasTimeInLowPercent"],
		expandedSorts["timeInLowRecords"]: expandedSorts["hasTimeInLowRecords"],
		expandedSorts["timeInLowMinutes"]: expandedSorts["hasTimeInLowMinutes"],

		expandedSorts["timeInVeryLowPercent"]: expandedSorts["hasTimeInVeryLowPercent"],
		expandedSorts["timeInVeryLowRecords"]: expandedSorts["hasTimeInVeryLowRecords"],
		expandedSorts["timeInVeryLowMinutes"]: expandedSorts["hasTimeInVeryLowMinutes"],

		expandedSorts["timeInHighPercent"]: expandedSorts["hasTimeInHighPercent"],
		expandedSorts["timeInHighRecords"]: expandedSorts["hasTimeInHighRecords"],
		expandedSorts["timeInHighMinutes"]: expandedSorts["hasTimeInHighMinutes"],

		expandedSorts["timeInVeryHighPercent"]: expandedSorts["hasTimeInVeryHighPercent"],
		expandedSorts["timeInVeryHighRecords"]: expandedSorts["hasTimeInVeryHighRecords"],
		expandedSorts["timeInVeryHighMinutes"]: expandedSorts["hasTimeInVeryHighMinutes"],
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
		"lastData":       {},
		"firstData":      {},
		"outdatedSince":  {},

		"timeCGMUsePercent":          {},
		"glucoseManagementIndicator": {},
		"averageGlucose":             {},

		"timeInLowPercent": {},
		"timeInLowRecords": {},
		"timeInLowMinutes": {},

		"timeInVeryLowPercent": {},
		"timeInVeryLowRecords": {},
		"timeInVeryLowMinutes": {},

		"timeInHighPercent": {},
		"timeInHighMinutes": {},
		"timeInHighRecords": {},

		"timeInVeryHighPercent": {},
		"timeInVeryHighRecords": {},
		"timeInVeryHighMinutes": {},

		"timeInTargetPercent": {},
		"timeInTargetRecords": {},
		"timeInTargetMinutes": {},

		"totalRecords":        {},
		"averageDailyRecords": {},
	},
	"bgm": {
		"fullName":       {},
		"birthDate":      {},
		"lastUploadDate": {},
		"lastData":       {},
		"firstData":      {},
		"outdatedSince":  {},

		"averageGlucose": {},

		"timeInLowPercent":      {},
		"timeInLowRecords":      {},
		"timeInVeryLowPercent":  {},
		"timeInVeryLowRecords":  {},
		"timeInHighPercent":     {},
		"timeInHighRecords":     {},
		"timeInVeryHighPercent": {},
		"timeInVeryHighRecords": {},
		"timeInTargetPercent":   {},
		"timeInTargetRecords":   {},

		"totalRecords":        {},
		"averageDailyRecords": {},
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

func parseRangeFilter(filters patients.SummaryFilters, field string, filter *string) (err error) {
	if filter == nil || *filter == "" {
		return
	}

	matches := rangeFilterRegex.FindStringSubmatch(*filter)
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

	filters[field] = patients.FilterPair{
		Cmp:   matches[1],
		Value: value,
	}

	return
}

func parseDateRangeFilter(filters patients.SummaryDateFilters, field string, min *time.Time, max *time.Time) (filterPair patients.FilterDatePair) {
	// normalize any Zero values to nil
	if min.IsZero() {
		min = nil
	}
	if max.IsZero() {
		max = nil
	}

	if min != nil || max != nil {
		filters[field] = patients.FilterDatePair{
			Min: min,
			Max: max,
		}
	}

	return
}

var validCmps = map[string]struct{}{
	">":  {},
	">=": {},
	"<":  {},
	"<=": {},
}

func patientTagsToObjectIds(tags *[]PatientTagId) *[]primitive.ObjectID {
	var tagIds []primitive.ObjectID
	if tags != nil {
		for _, id := range *tags {
			if tagId, err := primitive.ObjectIDFromHex(string(id)); err == nil {
				tagIds = append(tagIds, tagId)
			}
		}
	}
	return &tagIds
}

func ParseCGMSummaryFilters(params ListPatientsParams) (filters patients.SummaryFilters, err error) {
	filters = patients.SummaryFilters{}

	fieldsMap := map[string]*string{
		"timeCGMUsePercent":     params.CgmTimeCGMUsePercent,
		"timeInVeryLowPercent":  params.CgmTimeInVeryLowPercent,
		"timeInLowPercent":      params.CgmTimeInLowPercent,
		"timeInTargetPercent":   params.CgmTimeInTargetPercent,
		"timeInHighPercent":     params.CgmTimeInHighPercent,
		"timeInVeryHighPercent": params.CgmTimeInVeryHighPercent,

		"timeCGMUseRecords":     params.CgmTimeCGMUseRecords,
		"timeInVeryLowRecords":  params.CgmTimeInVeryLowRecords,
		"timeInLowRecords":      params.CgmTimeInLowRecords,
		"timeInTargetRecords":   params.CgmTimeInTargetRecords,
		"timeInHighRecords":     params.CgmTimeInHighRecords,
		"timeInVeryHighRecords": params.CgmTimeInVeryHighRecords,
		"averageDailyRecords":   params.CgmAverageDailyRecords,
		"totalRecords":          params.CgmTotalRecords,
	}

	for field, value := range fieldsMap {
		err = parseRangeFilter(filters, field, value)
		if err != nil {
			return
		}
	}

	return
}

func ParseBGMSummaryFilters(params ListPatientsParams) (filters patients.SummaryFilters, err error) {
	filters = patients.SummaryFilters{}

	fieldsMap := map[string]*string{
		"timeInVeryLowPercent":  params.BgmTimeInVeryLowPercent,
		"timeInLowPercent":      params.BgmTimeInLowPercent,
		"timeInTargetPercent":   params.BgmTimeInTargetPercent,
		"timeInHighPercent":     params.BgmTimeInHighPercent,
		"timeInVeryHighPercent": params.BgmTimeInVeryHighPercent,

		"timeInVeryLowRecords":  params.BgmTimeInVeryLowRecords,
		"timeInLowRecords":      params.BgmTimeInLowRecords,
		"timeInTargetRecords":   params.BgmTimeInTargetRecords,
		"timeInHighRecords":     params.BgmTimeInHighRecords,
		"timeInVeryHighRecords": params.BgmTimeInVeryHighRecords,
		"averageDailyRecords":   params.BgmAverageDailyRecords,
		"totalRecords":          params.BgmTotalRecords,
	}

	for field, value := range fieldsMap {
		err = parseRangeFilter(filters, field, value)
		if err != nil {
			return
		}
	}

	return
}

func ParseCGMSummaryDateFilters(params ListPatientsParams) (filters patients.SummaryDateFilters) {
	filters = patients.SummaryDateFilters{}

	parseDateRangeFilter(filters, "lastUploadDate", params.CgmLastUploadDateFrom, params.CgmLastUploadDateTo)
	return
}

func ParseBGMSummaryDateFilters(params ListPatientsParams) (filters patients.SummaryDateFilters) {
	filters = patients.SummaryDateFilters{}

	parseDateRangeFilter(filters, "lastUploadDate", params.BgmLastUploadDateFrom, params.BgmLastUploadDateTo)
	return
}
