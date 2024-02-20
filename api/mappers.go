package api

import (
	"fmt"
	"github.com/oapi-codegen/runtime/types"
	"regexp"
	"strconv"
	"strings"
	"time"

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

	clinic := &clinics.Clinic{
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
	if c.Timezone != nil {
		tz := string(*c.Timezone)
		clinic.Timezone = &tz
	}

	return clinic
}

func NewClinicDto(c *clinics.Clinic) Clinic {
	tier := clinics.DefaultTier
	if c.Tier != "" {
		tier = c.Tier
	}

	units := MgdL
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
	if c.Timezone != nil {
		tz := ClinicTimezone(*c.Timezone)
		dto.Timezone = &tz
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
		Id:          clinician.UserId,
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
		UserId:   clinician.Id,
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
		Id:            patient.UserId,
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

func NewPatientFromCreate(dto CreatePatient) patients.Patient {
	patient := patients.Patient{
		Permissions: NewPermissions(dto.Permissions),
	}
	if dto.BirthDate != nil {
		birthDate := dto.BirthDate.String()
		patient.BirthDate = &birthDate
	}
	if dto.FullName != nil {
		patient.FullName = dto.FullName
	}
	if dto.IsMigrated != nil {
		patient.IsMigrated = *dto.IsMigrated
	}
	if dto.LegacyClinicianId != nil {
		patient.LegacyClinicianIds = []string{*dto.LegacyClinicianId}
	}
	if dto.Mrn != nil && len(*dto.Mrn) > 0 {
		patient.Mrn = dto.Mrn
	}
	if dto.Tags != nil {
		tags := store.ObjectIDSFromStringArray(*dto.Tags)
		patient.Tags = &tags
	}
	return patient
}

func NewSummary(dto *PatientSummary) *patients.Summary {
	if dto == nil {
		return nil
	}

	patientSummary := &patients.Summary{}

	if dto.CgmStats != nil {
		patientSummary.CGM = &patients.PatientCGMStats{
			Periods:       patients.PatientCGMPeriods{},
			OffsetPeriods: patients.PatientCGMPeriods{},
			TotalHours:    dto.CgmStats.TotalHours,
		}

		patientSummary.CGM.Config = patients.PatientSummaryConfig(dto.CgmStats.Config)
		patientSummary.CGM.Dates = patients.PatientSummaryDates(dto.CgmStats.Dates)

		if dto.CgmStats.Periods != nil {
			for k, source := range dto.CgmStats.Periods {
				patientSummary.CGM.Periods[k] = patients.PatientCGMPeriod(source)
			}
		}

		if dto.CgmStats.OffsetPeriods != nil {
			for k, source := range dto.CgmStats.OffsetPeriods {
				patientSummary.CGM.OffsetPeriods[k] = patients.PatientCGMPeriod(source)
			}
		}
	}

	if dto.BgmStats != nil {
		patientSummary.BGM = &patients.PatientBGMStats{
			Periods:       patients.PatientBGMPeriods{},
			OffsetPeriods: patients.PatientBGMPeriods{},
			TotalHours:    dto.BgmStats.TotalHours,
		}

		patientSummary.BGM.Config = patients.PatientSummaryConfig(dto.BgmStats.Config)
		patientSummary.BGM.Dates = patients.PatientSummaryDates(dto.BgmStats.Dates)

		if dto.BgmStats.Periods != nil {
			for k, source := range dto.BgmStats.Periods {
				patientSummary.BGM.Periods[k] = patients.PatientBGMPeriod(source)
			}
		}

		if dto.BgmStats.OffsetPeriods != nil {
			for k, source := range dto.BgmStats.OffsetPeriods {
				patientSummary.BGM.OffsetPeriods[k] = patients.PatientBGMPeriod(source)
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
			Periods:       PatientCGMPeriods{},
			OffsetPeriods: PatientCGMPeriods{},
			TotalHours:    summary.CGM.TotalHours,
		}

		patientSummary.CgmStats.Config = PatientSummaryConfig(summary.CGM.Config)
		patientSummary.CgmStats.Dates = PatientSummaryDates(summary.CGM.Dates)

		if summary.CGM.Periods != nil {
			for k, source := range summary.CGM.Periods {
				patientSummary.CgmStats.Periods[k] = PatientCGMPeriod(source)
			}
		}

		if summary.CGM.OffsetPeriods != nil {
			for k, source := range summary.CGM.OffsetPeriods {
				patientSummary.CgmStats.OffsetPeriods[k] = PatientCGMPeriod(source)
			}
		}
	}

	if summary.BGM != nil {
		patientSummary.BgmStats = &PatientBGMStats{
			Periods:       PatientBGMPeriods{},
			OffsetPeriods: PatientBGMPeriods{},
			TotalHours:    summary.BGM.TotalHours,
		}

		patientSummary.BgmStats.Config = PatientSummaryConfig(summary.BGM.Config)
		patientSummary.BgmStats.Dates = PatientSummaryDates(summary.BGM.Dates)

		if summary.BGM.Periods != nil {
			for k, source := range summary.BGM.Periods {
				patientSummary.BgmStats.Periods[k] = PatientBGMPeriod(source)
			}
		}

		if summary.BGM.OffsetPeriods != nil {
			for k, source := range summary.BGM.OffsetPeriods {
				patientSummary.BgmStats.OffsetPeriods[k] = PatientBGMPeriod(source)
			}
		}
	}

	return patientSummary
}

func NewTideDto(tide *patients.Tide) *Tide {
	if tide == nil {
		return nil
	}

	tideResult := &Tide{
		Config: TideConfig{
			ClinicId:                 &tide.Config.ClinicId,
			Filters:                  TideFilters(tide.Config.Filters),
			HighGlucoseThreshold:     tide.Config.HighGlucoseThreshold,
			LastUploadDateFrom:       tide.Config.LastUploadDateFrom,
			LastUploadDateTo:         tide.Config.LastUploadDateTo,
			LowGlucoseThreshold:      tide.Config.LowGlucoseThreshold,
			Period:                   tide.Config.Period,
			SchemaVersion:            tide.Config.SchemaVersion,
			Tags:                     &tide.Config.Tags,
			VeryHighGlucoseThreshold: tide.Config.VeryHighGlucoseThreshold,
			VeryLowGlucoseThreshold:  tide.Config.VeryLowGlucoseThreshold,
		},
		Results: TideResults{},
	}

	for category, tidePatients := range tide.Results {
		c := make([]TideResultPatient, 0, 50)
		for _, patient := range *tidePatients {
			c = append(c, TideResultPatient{
				AverageGlucoseMmol:         patient.AverageGlucoseMmol,
				GlucoseManagementIndicator: patient.GlucoseManagementIndicator,
				Patient:                    TidePatient(patient.Patient),
				TimeCGMUseMinutes:          patient.TimeCGMUseMinutes,
				TimeCGMUsePercent:          patient.TimeCGMUsePercent,
				TimeInHighPercent:          patient.TimeInHighPercent,
				TimeInLowPercent:           patient.TimeInLowPercent,
				TimeInTargetPercent:        patient.TimeInTargetPercent,
				TimeInTargetPercentDelta:   patient.TimeInTargetPercentDelta,
				TimeInVeryHighPercent:      patient.TimeInVeryHighPercent,
				TimeInVeryLowPercent:       patient.TimeInVeryLowPercent,
			})
		}
		tideResult.Results[category] = c
	}

	return tideResult
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

func NewEHRSettings(dto EHRSettings) *clinics.EHRSettings {
	settings := &clinics.EHRSettings{
		Enabled:  dto.Enabled,
		SourceId: dto.SourceId,
		ProcedureCodes: clinics.EHRProcedureCodes{
			EnableSummaryReports:          dto.ProcedureCodes.EnableSummaryReports,
			DisableSummaryReports:         dto.ProcedureCodes.DisableSummaryReports,
			CreateAccount:                 dto.ProcedureCodes.CreateAccount,
			CreateAccountAndEnableReports: dto.ProcedureCodes.CreateAccountAndEnableReports,
		},
		MrnIdType: dto.MrnIdType,
		Provider:  string(dto.Provider),
	}
	if dto.DestinationIds != nil {
		settings.DestinationIds = &clinics.EHRDestinationIds{
			Flowsheet: dto.DestinationIds.Flowsheet,
			Notes:     dto.DestinationIds.Notes,
			Results:   dto.DestinationIds.Results,
		}
	}
	if dto.Facility != nil {
		settings.Facility = &clinics.EHRFacility{
			Name: dto.Facility.Name,
		}
	}

	return settings
}

func NewEHRSettingsDto(settings *clinics.EHRSettings) *EHRSettings {
	if settings == nil {
		return nil
	}

	dto := &EHRSettings{
		Enabled:  settings.Enabled,
		SourceId: settings.SourceId,
		ProcedureCodes: EHRProcedureCodes{
			EnableSummaryReports:          settings.ProcedureCodes.EnableSummaryReports,
			DisableSummaryReports:         settings.ProcedureCodes.DisableSummaryReports,
			CreateAccount:                 settings.ProcedureCodes.CreateAccount,
			CreateAccountAndEnableReports: settings.ProcedureCodes.CreateAccountAndEnableReports,
		},
		MrnIdType: settings.GetMrnIDType(),
		Provider:  EHRSettingsProvider(settings.Provider),
	}
	if settings.DestinationIds != nil {
		dto.DestinationIds = &EHRDestinationIds{
			Flowsheet: settings.DestinationIds.Flowsheet,
			Notes:     settings.DestinationIds.Notes,
			Results:   settings.DestinationIds.Results,
		}
	}
	if settings.Facility != nil {
		dto.Facility = &EHRFacility{
			Name: settings.Facility.Name,
		}
	}

	return dto
}

func ParseSort(sort *Sort, typ *string, period *string, offset *bool) ([]*store.Sort, error) {
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

	periodVersion := "periods"
	if offset != nil && *offset == true {
		periodVersion = "offsetPeriods"
	}

	result := store.Sort{}

	if strings.HasPrefix(*sort, "+") {
		result.Ascending = true
	} else if strings.HasPrefix(*sort, "-") {
		result.Ascending = false
	} else {
		return nil, fmt.Errorf("%w: invalid sort parameter, missing sort order", errors.BadRequest)
	}

	result.Attribute = (*sort)[1:]
	if result.Attribute == "" {
		return nil, fmt.Errorf("%w: invalid sort parameter, missing sort attribute", errors.BadRequest)
	} else if !isSortAttributeValid(result.Attribute, *typ) {
		return nil, fmt.Errorf("%w: invalid sort parameter, invalid sort attribute", errors.BadRequest)
	}

	expandedSorts := map[string]string{
		"lastUpdatedDate": "summary." + *typ + "Stats.dates.lastUpdatedDate",

		"hasLastUploadDate": "summary." + *typ + "Stats.dates.hasLastUploadDate",
		"lastUploadDate":    "summary." + *typ + "Stats.dates.lastUploadDate",

		"hasFirstData": "summary." + *typ + "Stats.dates.hasFirstData",
		"firstData":    "summary." + *typ + "Stats.dates.firstData",

		"hasLastData": "summary." + *typ + "Stats.dates.hasLastData",
		"lastData":    "summary." + *typ + "Stats.dates.lastData",

		"hasOutdatedSince": "summary." + *typ + "Stats.dates.hasOutdatedSince",
		"outdatedSince":    "summary." + *typ + "Stats.dates.outdatedSince",

		"hasAverageGlucoseMmol":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasAverageGlucoseMmol",
		"averageGlucoseMmol":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".averageGlucoseMmol",
		"averageGlucoseMmolDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".averageGlucoseMmolDelta",

		"hasGlucoseManagementIndicator":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasGlucoseManagementIndicator",
		"glucoseManagementIndicator":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".glucoseManagementIndicator",
		"glucoseManagementIndicatorDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".glucoseManagementIndicatorDelta",

		"hasTimeCGMUsePercent":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeCGMUsePercent",
		"timeCGMUsePercent":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeCGMUsePercent",
		"timeCGMUsePercentDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeCGMUsePercentDelta",

		"hasTimeCGMUseRecords":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeCGMUseRecords",
		"timeCGMUseRecords":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeCGMUseRecords",
		"timeCGMUseRecordsDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeCGMUseRecordsDelta",

		"hasTimeCGMUseMinutes":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeCGMUseMinutes",
		"timeCGMUseMinutes":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeCGMUseMinutes",
		"timeCGMUseMinutesDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeCGMUseMinutesDelta",

		"hasTimeInTargetPercent":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInTargetPercent",
		"timeInTargetPercent":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInTargetPercent",
		"timeInTargetPercentDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInTargetPercentDelta",

		"hasTimeInTargetRecords":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInTargetRecords",
		"timeInTargetRecords":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInTargetRecords",
		"timeInTargetRecordsDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInTargetRecordsDelta",

		"hasTimeInTargetMinutes":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInTargetMinutes",
		"timeInTargetMinutes":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInTargetMinutes",
		"timeInTargetMinutesDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInTargetMinutesDelta",

		"hasTimeInLowPercent":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInLowPercent",
		"timeInLowPercent":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInLowPercent",
		"timeInLowPercentDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInLowPercentDelta",

		"hasTimeInLowRecords":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInLowRecords",
		"timeInLowRecords":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInLowRecords",
		"timeInLowRecordsDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInLowRecordsDelta",

		"hasTimeInLowMinutes":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInLowMinutes",
		"timeInLowMinutes":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInLowMinutes",
		"timeInLowMinutesDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInLowMinutesDelta",

		"hasTimeInVeryLowPercent":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInVeryLowPercent",
		"timeInVeryLowPercent":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryLowPercent",
		"timeInVeryLowPercentDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryLowPercentDelta",

		"hasTimeInVeryLowRecords":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInVeryLowRecords",
		"timeInVeryLowRecords":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryLowRecords",
		"timeInVeryLowRecordsDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryLowRecordsDelta",

		"hasTimeInVeryLowMinutes":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInVeryLowMinutes",
		"timeInVeryLowMinutes":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryLowMinutes",
		"timeInVeryLowMinutesDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryLowMinutesDelta",

		"hasTimeInAnyLowPercent":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInAnyLowPercent",
		"timeInAnyLowPercent":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyLowPercent",
		"timeInAnyLowPercentDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyLowPercentDelta",

		"hasTimeInAnyLowRecords":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInAnyLowRecords",
		"timeInAnyLowRecords":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyLowRecords",
		"timeInAnyLowRecordsDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyLowRecordsDelta",

		"hasTimeInAnyLowMinutes":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInAnyLowMinutes",
		"timeInAnyLowMinutes":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyLowMinutes",
		"timeInAnyLowMinutesDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyLowMinutesDelta",

		"hasTimeInHighPercent":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInHighPercent",
		"timeInHighPercent":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInHighPercent",
		"timeInHighPercentDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInHighPercentDelta",

		"hasTimeInHighMinutes":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInHighMinutes",
		"timeInHighMinutes":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInHighMinutes",
		"timeInHighMinutesDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInHighMinutesDelta",

		"hasTimeInHighRecords":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInHighRecords",
		"timeInHighRecords":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInHighRecords",
		"timeInHighRecordsDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInHighRecordsDelta",

		"hasTimeInVeryHighPercent":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInVeryHighPercent",
		"timeInVeryHighPercent":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryHighPercent",
		"timeInVeryHighPercentDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryHighPercentDelta",

		"hasTimeInVeryHighRecords":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInVeryHighRecords",
		"timeInVeryHighRecords":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryHighRecords",
		"timeInVeryHighRecordsDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryHighRecordsDelta",

		"hasTimeInVeryHighMinutes":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInVeryHighMinutes",
		"timeInVeryHighMinutes":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryHighMinutes",
		"timeInVeryHighMinutesDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInVeryHighMinutesDelta",

		"hasTimeInAnyHighPercent":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInAnyHighPercent",
		"timeInAnyHighPercent":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyHighPercent",
		"timeInAnyHighPercentDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyHighPercentDelta",

		"hasTimeInAnyHighRecords":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInAnyHighRecords",
		"timeInAnyHighRecords":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyHighRecords",
		"timeInAnyHighRecordsDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyHighRecordsDelta",

		"hasTimeInAnyHighMinutes":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTimeInAnyHighMinutes",
		"timeInAnyHighMinutes":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyHighMinutes",
		"timeInAnyHighMinutesDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".timeInAnyHighMinutesDelta",

		"hasAverageDailyRecords":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasAverageDailyRecords",
		"averageDailyRecords":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".averageDailyRecords",
		"averageDailyRecordsDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".averageDailyRecordsDelta",

		"hasTotalRecords":   "summary." + *typ + "Stats." + periodVersion + "." + *period + ".hasTotalRecords",
		"totalRecords":      "summary." + *typ + "Stats." + periodVersion + "." + *period + ".totalRecords",
		"totalRecordsDelta": "summary." + *typ + "Stats." + periodVersion + "." + *period + ".totalRecordsDelta",
	}

	extraSort := map[string]string{
		expandedSorts["lastUploadDate"]: expandedSorts["hasLastUploadDate"],
		expandedSorts["lastData"]:       expandedSorts["hasLastData"],
		expandedSorts["firstData"]:      expandedSorts["hasFirstData"],
		expandedSorts["outdatedSince"]:  expandedSorts["hasOutdatedSince"],

		expandedSorts["glucoseManagementIndicator"]:      expandedSorts["hasGlucoseManagementIndicator"],
		expandedSorts["glucoseManagementIndicatorDelta"]: expandedSorts["hasGlucoseManagementIndicator"],

		expandedSorts["averageGlucoseMmol"]:      expandedSorts["hasAverageGlucoseMmol"],
		expandedSorts["averageGlucoseMmolDelta"]: expandedSorts["hasAverageGlucoseMmol"],

		expandedSorts["totalRecords"]:      expandedSorts["hasTotalRecords"],
		expandedSorts["totalRecordsDelta"]: expandedSorts["hasTotalRecords"],

		expandedSorts["averageDailyRecords"]:      expandedSorts["hasAverageDailyRecords"],
		expandedSorts["averageDailyRecordsDelta"]: expandedSorts["hasAverageDailyRecords"],

		expandedSorts["timeCGMUsePercent"]:      expandedSorts["hasTimeCGMUsePercent"],
		expandedSorts["timeCGMUsePercentDelta"]: expandedSorts["hasTimeCGMUsePercent"],
		expandedSorts["timeCGMUseRecords"]:      expandedSorts["hasTimeCGMUseRecords"],
		expandedSorts["timeCGMUseRecordsDelta"]: expandedSorts["hasTimeCGMUseRecords"],
		expandedSorts["timeCGMUseMinutes"]:      expandedSorts["hasTimeCGMUseMinutes"],
		expandedSorts["timeCGMUseMinutesDelta"]: expandedSorts["hasTimeCGMUseMinutes"],

		expandedSorts["timeInTargetPercent"]:      expandedSorts["hasTimeInTargetPercent"],
		expandedSorts["timeInTargetPercentDelta"]: expandedSorts["hasTimeInTargetPercent"],
		expandedSorts["timeInTargetRecords"]:      expandedSorts["hasTimeInTargetRecords"],
		expandedSorts["timeInTargetRecordsDelta"]: expandedSorts["hasTimeInTargetRecords"],
		expandedSorts["timeInTargetMinutes"]:      expandedSorts["hasTimeInTargetMinutes"],
		expandedSorts["timeInTargetMinutesDelta"]: expandedSorts["hasTimeInTargetMinutes"],

		expandedSorts["timeInLowPercent"]:      expandedSorts["hasTimeInLowPercent"],
		expandedSorts["timeInLowPercentDelta"]: expandedSorts["hasTimeInLowPercent"],
		expandedSorts["timeInLowRecords"]:      expandedSorts["hasTimeInLowRecords"],
		expandedSorts["timeInLowRecordsDelta"]: expandedSorts["hasTimeInLowRecords"],
		expandedSorts["timeInLowMinutes"]:      expandedSorts["hasTimeInLowMinutes"],
		expandedSorts["timeInLowMinutesDelta"]: expandedSorts["hasTimeInLowMinutes"],

		expandedSorts["timeInVeryLowPercent"]:      expandedSorts["hasTimeInVeryLowPercent"],
		expandedSorts["timeInVeryLowPercentDelta"]: expandedSorts["hasTimeInVeryLowPercent"],
		expandedSorts["timeInVeryLowRecords"]:      expandedSorts["hasTimeInVeryLowRecords"],
		expandedSorts["timeInVeryLowRecordsDelta"]: expandedSorts["hasTimeInVeryLowRecords"],
		expandedSorts["timeInVeryLowMinutes"]:      expandedSorts["hasTimeInVeryLowMinutes"],
		expandedSorts["timeInVeryLowMinutesDelta"]: expandedSorts["hasTimeInVeryLowMinutes"],

		expandedSorts["timeInAnyLowPercent"]:      expandedSorts["hasTimeInAnyLowPercent"],
		expandedSorts["timeInAnyLowPercentDelta"]: expandedSorts["hasTimeInAnyLowPercent"],
		expandedSorts["timeInAnyLowRecords"]:      expandedSorts["hasTimeInAnyLowRecords"],
		expandedSorts["timeInAnyLowRecordsDelta"]: expandedSorts["hasTimeInAnyLowRecords"],
		expandedSorts["timeInAnyLowMinutes"]:      expandedSorts["hasTimeInAnyLowMinutes"],
		expandedSorts["timeInAnyLowMinutesDelta"]: expandedSorts["hasTimeInAnyLowMinutes"],

		expandedSorts["timeInHighPercent"]:      expandedSorts["hasTimeInHighPercent"],
		expandedSorts["timeInHighPercentDelta"]: expandedSorts["hasTimeInHighPercent"],
		expandedSorts["timeInHighRecords"]:      expandedSorts["hasTimeInHighRecords"],
		expandedSorts["timeInHighRecordsDelta"]: expandedSorts["hasTimeInHighRecords"],
		expandedSorts["timeInHighMinutes"]:      expandedSorts["hasTimeInHighMinutes"],
		expandedSorts["timeInHighMinutesDelta"]: expandedSorts["hasTimeInHighMinutes"],

		expandedSorts["timeInVeryHighPercent"]:      expandedSorts["hasTimeInVeryHighPercent"],
		expandedSorts["timeInVeryHighPercentDelta"]: expandedSorts["hasTimeInVeryHighPercent"],
		expandedSorts["timeInVeryHighRecords"]:      expandedSorts["hasTimeInVeryHighRecords"],
		expandedSorts["timeInVeryHighRecordsDelta"]: expandedSorts["hasTimeInVeryHighRecords"],
		expandedSorts["timeInVeryHighMinutes"]:      expandedSorts["hasTimeInVeryHighMinutes"],
		expandedSorts["timeInVeryHighMinutesDelta"]: expandedSorts["hasTimeInVeryHighMinutes"],

		expandedSorts["timeInAnyHighPercent"]:      expandedSorts["hasTimeInAnyHighPercent"],
		expandedSorts["timeInAnyHighPercentDelta"]: expandedSorts["hasTimeInAnyHighPercent"],
		expandedSorts["timeInAnyHighRecords"]:      expandedSorts["hasTimeInAnyHighRecords"],
		expandedSorts["timeInAnyHighRecordsDelta"]: expandedSorts["hasTimeInAnyHighRecords"],
		expandedSorts["timeInAnyHighMinutes"]:      expandedSorts["hasTimeInAnyHighMinutes"],
		expandedSorts["timeInAnyHighMinutesDelta"]: expandedSorts["hasTimeInAnyHighMinutes"],
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

		"timeCGMUsePercent":      {},
		"timeCGMUsePercentDelta": {},

		"glucoseManagementIndicator":      {},
		"glucoseManagementIndicatorDelta": {},

		"averageGlucoseMmol":      {},
		"averageGlucoseMmolDelta": {},

		"timeInLowPercent":      {},
		"timeInLowPercentDelta": {},
		"timeInLowRecords":      {},
		"timeInLowRecordsDelta": {},
		"timeInLowMinutes":      {},
		"timeInLowMinutesDelta": {},

		"timeInVeryLowPercent":      {},
		"timeInVeryLowPercentDelta": {},
		"timeInVeryLowRecords":      {},
		"timeInVeryLowRecordsDelta": {},
		"timeInVeryLowMinutes":      {},
		"timeInVeryLowMinutesDelta": {},

		"timeInAnyLowPercent":      {},
		"timeInAnyLowPercentDelta": {},
		"timeInAnyLowRecords":      {},
		"timeInAnyLowRecordsDelta": {},
		"timeInAnyLowMinutes":      {},
		"timeInAnyLowMinutesDelta": {},

		"timeInHighPercent":      {},
		"timeInHighPercentDelta": {},
		"timeInHighMinutes":      {},
		"timeInHighMinutesDelta": {},
		"timeInHighRecords":      {},
		"timeInHighRecordsDelta": {},

		"timeInVeryHighPercent":      {},
		"timeInVeryHighPercentDelta": {},
		"timeInVeryHighRecords":      {},
		"timeInVeryHighRecordsDelta": {},
		"timeInVeryHighMinutes":      {},
		"timeInVeryHighMinutesDelta": {},

		"timeInAnyHighPercent":      {},
		"timeInAnyHighPercentDelta": {},
		"timeInAnyHighRecords":      {},
		"timeInAnyHighRecordsDelta": {},
		"timeInAnyHighMinutes":      {},
		"timeInAnyHighMinutesDelta": {},

		"timeInTargetPercent":      {},
		"timeInTargetPercentDelta": {},
		"timeInTargetRecords":      {},
		"timeInTargetRecordsDelta": {},
		"timeInTargetMinutes":      {},
		"timeInTargetMinutesDelta": {},

		"totalRecords":      {},
		"totalRecordsDelta": {},

		"averageDailyRecords":      {},
		"averageDailyRecordsDelta": {},
	},
	"bgm": {
		"fullName":       {},
		"birthDate":      {},
		"lastUploadDate": {},
		"lastData":       {},
		"firstData":      {},
		"outdatedSince":  {},

		"averageGlucoseMmol":      {},
		"averageGlucoseMmolDelta": {},

		"timeInLowPercent":      {},
		"timeInLowPercentDelta": {},
		"timeInLowRecords":      {},
		"timeInLowRecordsDelta": {},

		"timeInVeryLowPercent":      {},
		"timeInVeryLowPercentDelta": {},
		"timeInVeryLowRecords":      {},
		"timeInVeryLowRecordsDelta": {},

		"timeInAnyLowPercent":      {},
		"timeInAnyLowPercentDelta": {},
		"timeInAnyLowRecords":      {},
		"timeInAnyLowRecordsDelta": {},

		"timeInHighPercent":      {},
		"timeInHighPercentDelta": {},
		"timeInHighRecords":      {},
		"timeInHighRecordsDelta": {},

		"timeInVeryHighPercent":      {},
		"timeInVeryHighPercentDelta": {},
		"timeInVeryHighRecords":      {},
		"timeInVeryHighRecordsDelta": {},

		"timeInAnyHighPercent":      {},
		"timeInAnyHighPercentDelta": {},
		"timeInAnyHighRecords":      {},
		"timeInAnyHighRecordsDelta": {},

		"timeInTargetPercent":      {},
		"timeInTargetPercentDelta": {},
		"timeInTargetRecords":      {},
		"timeInTargetRecordsDelta": {},

		"totalRecords":      {},
		"totalRecordsDelta": {},

		"averageDailyRecords":      {},
		"averageDailyRecordsDelta": {},
	},
}

func isSortAttributeValid(attribute string, t string) bool {
	_, ok := validSortAttributes[t][attribute]
	return ok
}

const dateFormat = "2006-01-02"

func strtodatep(s *string) *types.Date {
	if s == nil {
		return nil
	}
	t, err := time.Parse(dateFormat, *s)
	if err != nil {
		return nil
	}
	return &types.Date{Time: t}
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
	if min != nil && min.IsZero() {
		min = nil
	}
	if max != nil && max.IsZero() {
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
		"timeInAnyLowPercent":   params.CgmTimeInAnyLowPercent,
		"timeInLowPercent":      params.CgmTimeInLowPercent,
		"timeInTargetPercent":   params.CgmTimeInTargetPercent,
		"timeInHighPercent":     params.CgmTimeInHighPercent,
		"timeInVeryHighPercent": params.CgmTimeInVeryHighPercent,
		"timeInAnyHighPercent":  params.CgmTimeInAnyHighPercent,

		"timeCGMUseRecords":     params.CgmTimeCGMUseRecords,
		"timeInVeryLowRecords":  params.CgmTimeInVeryLowRecords,
		"timeInAnyLowRecords":   params.CgmTimeInAnyLowRecords,
		"timeInLowRecords":      params.CgmTimeInLowRecords,
		"timeInTargetRecords":   params.CgmTimeInTargetRecords,
		"timeInHighRecords":     params.CgmTimeInHighRecords,
		"timeInVeryHighRecords": params.CgmTimeInVeryHighRecords,
		"timeInAnyHighRecords":  params.CgmTimeInAnyHighRecords,
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
		"timeInAnyLowPercent":   params.BgmTimeInAnyLowPercent,
		"timeInLowPercent":      params.BgmTimeInLowPercent,
		"timeInTargetPercent":   params.BgmTimeInTargetPercent,
		"timeInHighPercent":     params.BgmTimeInHighPercent,
		"timeInVeryHighPercent": params.BgmTimeInVeryHighPercent,
		"timeInAnyHighPercent":  params.BgmTimeInAnyHighPercent,

		"timeInVeryLowRecords":  params.BgmTimeInVeryLowRecords,
		"timeInAnyLowRecords":   params.BgmTimeInAnyLowRecords,
		"timeInLowRecords":      params.BgmTimeInLowRecords,
		"timeInTargetRecords":   params.BgmTimeInTargetRecords,
		"timeInHighRecords":     params.BgmTimeInHighRecords,
		"timeInVeryHighRecords": params.BgmTimeInVeryHighRecords,
		"timeInAnyHighRecords":  params.BgmTimeInAnyHighRecords,
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
