package api

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/migration"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func NewClinicWithDefaults(c ClinicV1) *clinics.Clinic {
	clinic := mapClinic(c, clinics.NewClinicWithDefaults())
	clinic.UpdatePatientCountSettingsForCountry()
	return clinic
}

func NewClinic(c ClinicV1) *clinics.Clinic {
	return mapClinic(c, clinics.NewClinic())
}

func mapClinic(c ClinicV1, clinic *clinics.Clinic) *clinics.Clinic {
	var phoneNumbers []clinics.PhoneNumber
	if c.PhoneNumbers != nil {
		for _, n := range *c.PhoneNumbers {
			phoneNumbers = append(phoneNumbers, clinics.PhoneNumber{
				Number: n.Number,
				Type:   n.Type,
			})
		}
	}

	clinic.Name = &c.Name
	clinic.ClinicType = clinicTypeToString(c.ClinicType)
	clinic.ClinicSize = clinicSizeToString(c.ClinicSize)
	clinic.Address = c.Address
	clinic.City = c.City
	clinic.Country = c.Country
	clinic.PostalCode = c.PostalCode
	clinic.State = c.State
	clinic.PhoneNumbers = &phoneNumbers
	clinic.Website = c.Website
	clinic.PreferredBgUnits = string(c.PreferredBgUnits)
	if c.Timezone != nil {
		tz := string(*c.Timezone)
		clinic.Timezone = &tz
	}

	return clinic
}

func NewClinicDto(c *clinics.Clinic) ClinicV1 {
	tier := clinics.DefaultTier
	if c.Tier != "" {
		tier = c.Tier
	}

	units := MgdL
	if c.PreferredBgUnits != "" {
		units = ClinicV1PreferredBgUnits(c.PreferredBgUnits)
	}
	id := ClinicIdV1(c.Id.Hex())
	canMigrate := c.CanMigrate()

	dto := ClinicV1{
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
		SuppressedNotifications: (*SuppressedNotificationsV1)(c.SuppressedNotifications),
	}
	if c.PhoneNumbers != nil {
		var phoneNumbers []PhoneNumberV1
		for _, n := range *c.PhoneNumbers {
			phoneNumbers = append(phoneNumbers, PhoneNumberV1{
				Number: n.Number,
				Type:   n.Type,
			})
		}
		dto.PhoneNumbers = &phoneNumbers
	}
	if c.PatientTags != nil {
		var patientTags []PatientTagV1
		for _, n := range c.PatientTags {
			patientTags = append(patientTags, PatientTagV1{
				Id:   strp(n.Id.Hex()),
				Name: n.Name,
			})
		}
		dto.PatientTags = &patientTags
	}
	if c.Timezone != nil {
		tz := ClinicTimezoneV1(*c.Timezone)
		dto.Timezone = &tz
	}

	return dto
}

func NewClinicsDto(clinics []*clinics.Clinic) []ClinicV1 {
	dtos := make([]ClinicV1, 0)
	for _, clinic := range clinics {
		dtos = append(dtos, NewClinicDto(clinic))
	}
	return dtos
}

func NewClinicianDto(clinician *clinicians.Clinician) ClinicianV1 {
	dto := ClinicianV1{
		Id:          clinician.UserId,
		InviteId:    clinician.InviteId,
		Name:        clinician.Name,
		Email:       pstr(clinician.Email),
		Roles:       ClinicianRolesV1(clinician.Roles),
		CreatedTime: &clinician.CreatedTime,
		UpdatedTime: &clinician.UpdatedTime,
	}
	return dto
}

func NewCliniciansDto(clinicians []*clinicians.Clinician) []ClinicianV1 {
	dtos := make([]ClinicianV1, 0)
	for _, c := range clinicians {
		if c != nil {
			dtos = append(dtos, NewClinicianDto(c))
		}
	}
	return dtos
}

func NewClinician(clinician ClinicianV1) *clinicians.Clinician {
	return &clinicians.Clinician{
		Name:     clinician.Name,
		UserId:   clinician.Id,
		InviteId: clinician.InviteId,
		Roles:    clinician.Roles,
		Email:    strp(strings.ToLower(clinician.Email)),
	}
}

func NewClinicianUpdate(clinician ClinicianV1) clinicians.Clinician {
	return clinicians.Clinician{
		Name:  clinician.Name,
		Roles: clinician.Roles,
	}
}

func NewPatientDto(patient *patients.Patient) PatientV1 {
	dto := PatientV1{
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
		Reviews:       NewReviewsDto(patient.Reviews),
		ConnectionRequests: &ProviderConnectionRequestsV1{
			Abbott: NewConnectionRequestDTO(patient.ProviderConnectionRequests, Abbott),
			Dexcom: NewConnectionRequestDTO(patient.ProviderConnectionRequests, Dexcom),
			Twiist: NewConnectionRequestDTO(patient.ProviderConnectionRequests, Twiist),
		},
	}
	if patient.BirthDate != nil && strtodatep(patient.BirthDate) != nil {
		dto.BirthDate = *strtodatep(patient.BirthDate)
	}
	if !patient.LastUploadReminderTime.IsZero() {
		dto.LastUploadReminderTime = &patient.LastUploadReminderTime
	}

	// Populate the new connection requests structure from the now deprecated lastRequestedDexcomConnectTime
	if len(dto.ConnectionRequests.Dexcom) == 0 && !patient.LastRequestedDexcomConnectTime.IsZero() {
		dto.ConnectionRequests.Dexcom = []ProviderConnectionRequestV1{{
			ProviderName: Dexcom,
			CreatedTime:  patient.LastRequestedDexcomConnectTime,
		}}
	}

	return dto
}

func NewConnectionRequestDTO(requests patients.ProviderConnectionRequests, provider ProviderId) []ProviderConnectionRequestV1 {
	var requestsForProvider patients.ConnectionRequests
	if requests != nil {
		requestsForProvider = requests[string(provider)]
	}
	result := make([]ProviderConnectionRequestV1, len(requestsForProvider))
	for i, request := range requestsForProvider {
		result[i] = ProviderConnectionRequestV1{
			CreatedTime:  request.CreatedTime,
			ProviderName: ProviderId(request.ProviderName),
		}
	}
	return result
}

func NewPatient(dto PatientV1) patients.Patient {
	patient := patients.Patient{
		Email:         pstrToLower(dto.Email),
		BirthDate:     strp(dto.BirthDate.Format(dateFormat)),
		FullName:      &dto.FullName,
		Mrn:           dto.Mrn,
		TargetDevices: dto.TargetDevices,
		Reviews:       NewReviews(dto.Reviews),
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

func NewPatientFromCreate(dto CreatePatientV1) patients.Patient {
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

func NewSummary(dto *PatientSummaryV1) *patients.Summary {
	if dto == nil {
		return nil
	}

	patientSummary := &patients.Summary{}

	if dto.CgmStats != nil && dto.CgmStats.Id != nil {
		patientSummary.CGM = &patients.PatientCGMStats{
			Id:      *dto.CgmStats.Id,
			Periods: patients.PatientCGMPeriods{},
		}

		patientSummary.CGM.Config = patients.PatientSummaryConfig(dto.CgmStats.Config)
		patientSummary.CGM.Dates = patients.PatientSummaryDates(dto.CgmStats.Dates)

		if dto.CgmStats.Periods != nil {
			for k, source := range dto.CgmStats.Periods {
				patientSummary.CGM.Periods[k] = patients.PatientCGMPeriod(source)
			}
		}
	}

	if dto.BgmStats != nil && dto.BgmStats.Id != nil {
		patientSummary.BGM = &patients.PatientBGMStats{
			Id:      *dto.BgmStats.Id,
			Periods: patients.PatientBGMPeriods{},
		}

		patientSummary.BGM.Config = patients.PatientSummaryConfig(dto.BgmStats.Config)
		patientSummary.BGM.Dates = patients.PatientSummaryDates(dto.BgmStats.Dates)

		if dto.BgmStats.Periods != nil {
			for k, source := range dto.BgmStats.Periods {
				patientSummary.BGM.Periods[k] = patients.PatientBGMPeriod(source)
			}
		}
	}

	return patientSummary
}

func NewSummaryDto(summary *patients.Summary) *PatientSummaryV1 {
	if summary == nil {
		return nil
	}

	patientSummary := &PatientSummaryV1{}

	if summary.CGM != nil {
		patientSummary.CgmStats = &CgmStatsV1{
			Id:      &summary.CGM.Id,
			Periods: CgmPeriodsV1{},
		}

		patientSummary.CgmStats.Config = SummaryConfigV1(summary.CGM.Config)
		patientSummary.CgmStats.Dates = SummaryDatesV1(summary.CGM.Dates)

		if summary.CGM.Periods != nil {
			for k, source := range summary.CGM.Periods {
				patientSummary.CgmStats.Periods[k] = CgmPeriodV1(source)
			}
		}
	}

	if summary.BGM != nil {
		patientSummary.BgmStats = &BgmStatsV1{
			Id:      &summary.BGM.Id,
			Periods: BgmPeriodsV1{},
		}

		patientSummary.BgmStats.Config = SummaryConfigV1(summary.BGM.Config)
		patientSummary.BgmStats.Dates = SummaryDatesV1(summary.BGM.Dates)

		if summary.BGM.Periods != nil {
			for k, source := range summary.BGM.Periods {
				patientSummary.BgmStats.Periods[k] = BgmPeriodV1(source)
			}
		}
	}

	return patientSummary
}

func NewReviewDto(review patients.Review) PatientReviewV1 {
	return PatientReviewV1(review)
}

func NewReview(review PatientReviewV1) patients.Review {
	return patients.Review(review)
}

func NewReviewsDto(reviews []patients.Review) PatientReviewsV1 {
	result := make(PatientReviewsV1, len(reviews))
	for i := 0; i < len(reviews); i++ {
		result[i] = NewReviewDto(reviews[i])
	}
	return result
}

func NewReviews(reviews PatientReviewsV1) []patients.Review {
	result := make([]patients.Review, len(reviews))
	for i := 0; i < len(reviews); i++ {
		result[i] = NewReview(reviews[i])
	}
	return result
}

func NewTideDto(tide *patients.Tide) *TideResponseV1 {
	if tide == nil {
		return nil
	}

	tideResult := &TideResponseV1{
		Config: TideConfigV1{
			ClinicId:                 &tide.Config.ClinicId,
			Filters:                  TideFiltersV1(tide.Config.Filters),
			HighGlucoseThreshold:     tide.Config.HighGlucoseThreshold,
			LastDataCutoff:           tide.Config.LastDataCutoff,
			LowGlucoseThreshold:      tide.Config.LowGlucoseThreshold,
			Period:                   tide.Config.Period,
			SchemaVersion:            tide.Config.SchemaVersion,
			Tags:                     &tide.Config.Tags,
			VeryHighGlucoseThreshold: tide.Config.VeryHighGlucoseThreshold,
			VeryLowGlucoseThreshold:  tide.Config.VeryLowGlucoseThreshold,
		},
		Results: TideResultsV1{},
	}

	for category, tidePatients := range tide.Results {
		c := make([]TideResultPatientV1, 0, 50)
		for _, patient := range tidePatients {
			c = append(c, TideResultPatientV1{
				AverageGlucoseMmol:         patient.AverageGlucoseMmol,
				GlucoseManagementIndicator: patient.GlucoseManagementIndicator,
				TimeCGMUseMinutes:          patient.TimeCGMUseMinutes,
				TimeCGMUsePercent:          patient.TimeCGMUsePercent,
				TimeInHighPercent:          patient.TimeInHighPercent,
				TimeInLowPercent:           patient.TimeInLowPercent,
				TimeInTargetPercent:        patient.TimeInTargetPercent,
				TimeInTargetPercentDelta:   patient.TimeInTargetPercentDelta,
				TimeInVeryHighPercent:      patient.TimeInVeryHighPercent,
				TimeInVeryLowPercent:       patient.TimeInVeryLowPercent,
				TimeInAnyLowPercent:        patient.TimeInAnyLowPercent,
				TimeInAnyHighPercent:       patient.TimeInAnyHighPercent,
				LastData:                   patient.LastData,
				Patient: TidePatientV1{
					Email:       patient.Patient.Email,
					FullName:    patient.Patient.FullName,
					Id:          patient.Patient.Id,
					Tags:        &patient.Patient.Tags,
					Reviews:     NewReviewsDto(patient.Patient.Reviews),
					DataSources: NewPatientDataSourcesDto(patient.Patient.DataSources),
				},
			})
		}
		tideResult.Results[category] = c
	}

	return tideResult
}

func NewPermissions(dto *PatientPermissionsV1) *patients.Permissions {
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

func NewPermissionsDto(dto *patients.Permissions) *PatientPermissionsV1 {
	var permissions *PatientPermissionsV1
	if dto != nil {
		permissions = &PatientPermissionsV1{}
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

func NewPatientDataSourcesDto(dataSources *[]patients.DataSource) *[]DataSourceV1 {
	if dataSources == nil {
		return nil
	}

	dtos := make([]DataSourceV1, 0)

	if dataSources != nil {
		for _, d := range *dataSources {
			newDataSource := DataSourceV1{
				ProviderName: d.ProviderName,
				State:        DataSourceV1State(d.State),
			}

			if d.DataSourceId != nil {
				dataSourceId := d.DataSourceId.Hex()
				newDataSource.DataSourceId = &dataSourceId
			}

			if d.ModifiedTime != nil {
				modifiedTime := DatetimeV1(d.ModifiedTime.Format(time.RFC3339Nano))
				newDataSource.ModifiedTime = &modifiedTime
			}

			if d.ExpirationTime != nil {
				expirationTime := DatetimeV1(d.ExpirationTime.Format(time.RFC3339Nano))
				newDataSource.ExpirationTime = &expirationTime
			}

			dtos = append(dtos, newDataSource)
		}
	}

	return &dtos
}

func NewPatientsDto(patients []*patients.Patient) []PatientV1 {
	dtos := make([]PatientV1, 0, len(patients))
	for _, p := range patients {
		if p != nil {
			dtos = append(dtos, NewPatientDto(p))
		}
	}
	return dtos
}

func NewPatientsResponseDto(list *patients.ListResult, totalCount int) PatientsResponseV1 {
	data := PatientsV1(NewPatientsDto(list.Patients))
	return PatientsResponseV1{
		Data: &data,
		Meta: &MetaV1{
			Count:      &list.MatchingCount,
			TotalCount: &totalCount,
		},
	}
}

func NewPatientClinicRelationshipsDto(patients []*patients.Patient, clinicList []*clinics.Clinic) (PatientClinicRelationshipsV1, error) {
	clinicsMap := make(map[string]*clinics.Clinic, 0)
	for _, clinic := range clinicList {
		clinicsMap[clinic.Id.Hex()] = clinic
	}
	dtos := make([]PatientClinicRelationshipV1, 0)
	for _, patient := range patients {
		clinic, ok := clinicsMap[patient.ClinicId.Hex()]
		if !ok || clinic == nil {
			return nil, fmt.Errorf("clinic not found")
		}

		dtos = append(dtos, PatientClinicRelationshipV1{
			Clinic:  NewClinicDto(clinic),
			Patient: NewPatientDto(patient),
		})
	}
	return dtos, nil
}

func NewClinicianClinicRelationshipsDto(clinicians []*clinicians.Clinician, clinicList []*clinics.Clinic) (ClinicianClinicRelationshipsV1, error) {
	clinicsMap := make(map[string]*clinics.Clinic, 0)
	for _, clinic := range clinicList {
		clinicsMap[clinic.Id.Hex()] = clinic
	}
	dtos := make([]ClinicianClinicRelationshipV1, 0)
	for _, clinician := range clinicians {
		clinic, ok := clinicsMap[clinician.ClinicId.Hex()]
		if !ok || clinic == nil {
			return nil, fmt.Errorf("clinic not found")
		}

		dtos = append(dtos, ClinicianClinicRelationshipV1{
			Clinic:    NewClinicDto(clinic),
			Clinician: NewClinicianDto(clinician),
		})
	}

	return dtos, nil
}

func NewMigrationDto(migration *migration.Migration) *MigrationV1 {
	if migration == nil {
		return nil
	}

	result := &MigrationV1{
		CreatedTime: &migration.CreatedTime,
		UpdatedTime: &migration.UpdatedTime,
		UserId:      migration.UserId,
	}
	if migration.Status != "" {
		status := MigrationStatusV1(strings.ToUpper(migration.Status))
		result.Status = &status
	}
	return result
}

func NewMigrationDtos(migrations []*migration.Migration) []*MigrationV1 {
	var dtos []*MigrationV1
	if len(migrations) == 0 {
		return dtos
	}

	for _, m := range migrations {
		dtos = append(dtos, NewMigrationDto(m))
	}

	return dtos
}

func NewMembershipRestrictionsDto(restrictions []clinics.MembershipRestrictions) MembershipRestrictionsV1 {
	dto := MembershipRestrictionsV1{}
	var dtos []MembershipRestrictionV1
	for _, r := range restrictions {
		restriction := MembershipRestrictionV1{}
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

func NewMembershipRestrictions(dto MembershipRestrictionsV1) []clinics.MembershipRestrictions {
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

func NewEHRSettings(dto EhrSettingsV1) *clinics.EHRSettings {
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
		ScheduledReports: clinics.ScheduledReports{
			Cadence:         string(dto.ScheduledReports.Cadence),
			OnUploadEnabled: dto.ScheduledReports.OnUploadEnabled,
		},
		Tags: clinics.TagsSettings{
			Separator: dto.Tags.Separator,
		},
		Flowsheets: clinics.FlowsheetSettings{
			Icode: dto.Flowsheets.Icode,
		},
	}
	if settings.ScheduledReports.OnUploadEnabled && dto.ScheduledReports.OnUploadNoteEventType != nil {
		settings.ScheduledReports.OnUploadNoteEventType = strp(string(*dto.ScheduledReports.OnUploadNoteEventType))
	}
	if dto.Tags.Codes != nil {
		settings.Tags.Codes = *dto.Tags.Codes
	}
	if dto.DestinationIds != nil {
		settings.DestinationIds = &clinics.EHRDestinationIds{
			Flowsheet: dto.DestinationIds.Flowsheet,
			Notes:     dto.DestinationIds.Notes,
			Results:   dto.DestinationIds.Results,
		}
	}

	return settings
}

func NewEHRSettingsDto(settings *clinics.EHRSettings) *EhrSettingsV1 {
	if settings == nil {
		return nil
	}

	dto := &EhrSettingsV1{
		Enabled:  settings.Enabled,
		SourceId: settings.SourceId,
		ProcedureCodes: EhrProceduresV1{
			EnableSummaryReports:          settings.ProcedureCodes.EnableSummaryReports,
			DisableSummaryReports:         settings.ProcedureCodes.DisableSummaryReports,
			CreateAccount:                 settings.ProcedureCodes.CreateAccount,
			CreateAccountAndEnableReports: settings.ProcedureCodes.CreateAccountAndEnableReports,
		},
		MrnIdType: settings.GetMrnIDType(),
		Provider:  EhrSettingsV1Provider(settings.Provider),
		ScheduledReports: ScheduledReportsV1{
			OnUploadEnabled: settings.ScheduledReports.OnUploadEnabled,
		},
		Tags: EhrTagsSettingsV1{
			Codes:     &settings.Tags.Codes,
			Separator: settings.Tags.Separator,
		},
		Flowsheets: EhrFlowsheetSettingsV1{
			Icode: settings.Flowsheets.Icode,
		},
	}
	if settings.ScheduledReports.OnUploadNoteEventType != nil {
		eventType := ScheduledReportsV1OnUploadNoteEventType(*settings.ScheduledReports.OnUploadNoteEventType)
		dto.ScheduledReports.OnUploadNoteEventType = &eventType
	}
	if settings.DestinationIds != nil {
		dto.DestinationIds = &EhrDestinationsV1{
			Flowsheet: settings.DestinationIds.Flowsheet,
			Notes:     settings.DestinationIds.Notes,
			Results:   settings.DestinationIds.Results,
		}
	}
	if settings.ScheduledReports.Cadence != "" {
		dto.ScheduledReports.Cadence = ScheduledReportsV1Cadence(settings.ScheduledReports.Cadence)
	} else {
		// Default to 14 days
		dto.ScheduledReports.Cadence = N14d
	}
	return dto
}

func NewPatientCountSettings(dto PatientCountSettingsV1) *clinics.PatientCountSettings {
	return &clinics.PatientCountSettings{
		HardLimit: NewPatientCountLimit(dto.HardLimit),
		SoftLimit: NewPatientCountLimit(dto.SoftLimit),
	}
}

func NewPatientCountSettingsDto(settings *clinics.PatientCountSettings) *PatientCountSettingsV1 {
	if settings == nil {
		return nil
	}

	return &PatientCountSettingsV1{
		HardLimit: NewPatientCountLimitDto(settings.HardLimit),
		SoftLimit: NewPatientCountLimitDto(settings.SoftLimit),
	}
}

func NewPatientCountLimit(dto *PatientCountLimitV1) *clinics.PatientCountLimit {
	if dto == nil {
		return nil
	}

	patientCountLimit := &clinics.PatientCountLimit{
		PatientCount: dto.PatientCount,
	}

	if dto.StartDate != nil {
		startDate, _ := time.Parse(time.RFC3339Nano, string(*dto.StartDate))
		patientCountLimit.StartDate = &startDate
	}
	if dto.EndDate != nil {
		endDate, _ := time.Parse(time.RFC3339Nano, string(*dto.EndDate))
		patientCountLimit.EndDate = &endDate
	}

	return patientCountLimit
}

func NewPatientCountLimitDto(limit *clinics.PatientCountLimit) *PatientCountLimitV1 {
	if limit == nil {
		return nil
	}

	dto := &PatientCountLimitV1{
		PatientCount: limit.PatientCount,
	}

	if limit.StartDate != nil {
		startDate := limit.StartDate.Format(time.RFC3339Nano)
		dto.StartDate = &startDate
	}
	if limit.EndDate != nil {
		endDate := limit.EndDate.Format(time.RFC3339Nano)
		dto.EndDate = &endDate
	}

	return dto
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
		"lastReviewed": "reviews.0.time",

		"lastUpdatedDate": "summary." + *typ + "Stats.dates.lastUpdatedDate",

		"hasLastUploadDate": "summary." + *typ + "Stats.dates.hasLastUploadDate",
		"lastUploadDate":    "summary." + *typ + "Stats.dates.lastUploadDate",

		"hasFirstData": "summary." + *typ + "Stats.dates.hasFirstData",
		"firstData":    "summary." + *typ + "Stats.dates.firstData",

		"hasLastData": "summary." + *typ + "Stats.dates.hasLastData",
		"lastData":    "summary." + *typ + "Stats.dates.lastData",

		"hasOutdatedSince": "summary." + *typ + "Stats.dates.hasOutdatedSince",
		"outdatedSince":    "summary." + *typ + "Stats.dates.outdatedSince",

		"min": "summary." + *typ + "Stats.periods." + *period + ".min",
		"max": "summary." + *typ + "Stats.periods." + *period + ".max",

		"minDelta": "summary." + *typ + "Stats.periods." + *period + ".minDelta",
		"maxDelta": "summary." + *typ + "Stats.periods." + *period + ".maxDelta",

		"hasAverageGlucoseMmol":   "summary." + *typ + "Stats.periods." + *period + ".hasAverageGlucoseMmol",
		"averageGlucoseMmol":      "summary." + *typ + "Stats.periods." + *period + ".averageGlucoseMmol",
		"averageGlucoseMmolDelta": "summary." + *typ + "Stats.periods." + *period + ".averageGlucoseMmolDelta",

		"hasGlucoseManagementIndicator":   "summary." + *typ + "Stats.periods." + *period + ".hasGlucoseManagementIndicator",
		"glucoseManagementIndicator":      "summary." + *typ + "Stats.periods." + *period + ".glucoseManagementIndicator",
		"glucoseManagementIndicatorDelta": "summary." + *typ + "Stats.periods." + *period + ".glucoseManagementIndicatorDelta",

		"hasTimeCGMUsePercent":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeCGMUsePercent",
		"timeCGMUsePercent":      "summary." + *typ + "Stats.periods." + *period + ".timeCGMUsePercent",
		"timeCGMUsePercentDelta": "summary." + *typ + "Stats.periods." + *period + ".timeCGMUsePercentDelta",

		"hasTimeCGMUseRecords":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeCGMUseRecords",
		"timeCGMUseRecords":      "summary." + *typ + "Stats.periods." + *period + ".timeCGMUseRecords",
		"timeCGMUseRecordsDelta": "summary." + *typ + "Stats.periods." + *period + ".timeCGMUseRecordsDelta",

		"hasTimeCGMUseMinutes":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeCGMUseMinutes",
		"timeCGMUseMinutes":      "summary." + *typ + "Stats.periods." + *period + ".timeCGMUseMinutes",
		"timeCGMUseMinutesDelta": "summary." + *typ + "Stats.periods." + *period + ".timeCGMUseMinutesDelta",

		"hasTimeInTargetPercent":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInTargetPercent",
		"timeInTargetPercent":      "summary." + *typ + "Stats.periods." + *period + ".timeInTargetPercent",
		"timeInTargetPercentDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInTargetPercentDelta",

		"hasTimeInTargetRecords":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInTargetRecords",
		"timeInTargetRecords":      "summary." + *typ + "Stats.periods." + *period + ".timeInTargetRecords",
		"timeInTargetRecordsDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInTargetRecordsDelta",

		"hasTimeInTargetMinutes":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInTargetMinutes",
		"timeInTargetMinutes":      "summary." + *typ + "Stats.periods." + *period + ".timeInTargetMinutes",
		"timeInTargetMinutesDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInTargetMinutesDelta",

		"hasTimeInLowPercent":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInLowPercent",
		"timeInLowPercent":      "summary." + *typ + "Stats.periods." + *period + ".timeInLowPercent",
		"timeInLowPercentDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInLowPercentDelta",

		"hasTimeInLowRecords":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInLowRecords",
		"timeInLowRecords":      "summary." + *typ + "Stats.periods." + *period + ".timeInLowRecords",
		"timeInLowRecordsDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInLowRecordsDelta",

		"hasTimeInLowMinutes":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInLowMinutes",
		"timeInLowMinutes":      "summary." + *typ + "Stats.periods." + *period + ".timeInLowMinutes",
		"timeInLowMinutesDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInLowMinutesDelta",

		"hasTimeInVeryLowPercent":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryLowPercent",
		"timeInVeryLowPercent":      "summary." + *typ + "Stats.periods." + *period + ".timeInVeryLowPercent",
		"timeInVeryLowPercentDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInVeryLowPercentDelta",

		"hasTimeInVeryLowRecords":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryLowRecords",
		"timeInVeryLowRecords":      "summary." + *typ + "Stats.periods." + *period + ".timeInVeryLowRecords",
		"timeInVeryLowRecordsDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInVeryLowRecordsDelta",

		"hasTimeInVeryLowMinutes":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryLowMinutes",
		"timeInVeryLowMinutes":      "summary." + *typ + "Stats.periods." + *period + ".timeInVeryLowMinutes",
		"timeInVeryLowMinutesDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInVeryLowMinutesDelta",

		"hasTimeInAnyLowPercent":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInAnyLowPercent",
		"timeInAnyLowPercent":      "summary." + *typ + "Stats.periods." + *period + ".timeInAnyLowPercent",
		"timeInAnyLowPercentDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInAnyLowPercentDelta",

		"hasTimeInAnyLowRecords":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInAnyLowRecords",
		"timeInAnyLowRecords":      "summary." + *typ + "Stats.periods." + *period + ".timeInAnyLowRecords",
		"timeInAnyLowRecordsDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInAnyLowRecordsDelta",

		"hasTimeInAnyLowMinutes":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInAnyLowMinutes",
		"timeInAnyLowMinutes":      "summary." + *typ + "Stats.periods." + *period + ".timeInAnyLowMinutes",
		"timeInAnyLowMinutesDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInAnyLowMinutesDelta",

		"hasTimeInHighPercent":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInHighPercent",
		"timeInHighPercent":      "summary." + *typ + "Stats.periods." + *period + ".timeInHighPercent",
		"timeInHighPercentDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInHighPercentDelta",

		"hasTimeInHighMinutes":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInHighMinutes",
		"timeInHighMinutes":      "summary." + *typ + "Stats.periods." + *period + ".timeInHighMinutes",
		"timeInHighMinutesDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInHighMinutesDelta",

		"hasTimeInHighRecords":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInHighRecords",
		"timeInHighRecords":      "summary." + *typ + "Stats.periods." + *period + ".timeInHighRecords",
		"timeInHighRecordsDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInHighRecordsDelta",

		"hasTimeInVeryHighPercent":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryHighPercent",
		"timeInVeryHighPercent":      "summary." + *typ + "Stats.periods." + *period + ".timeInVeryHighPercent",
		"timeInVeryHighPercentDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInVeryHighPercentDelta",

		"hasTimeInVeryHighRecords":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryHighRecords",
		"timeInVeryHighRecords":      "summary." + *typ + "Stats.periods." + *period + ".timeInVeryHighRecords",
		"timeInVeryHighRecordsDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInVeryHighRecordsDelta",

		"hasTimeInVeryHighMinutes":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInVeryHighMinutes",
		"timeInVeryHighMinutes":      "summary." + *typ + "Stats.periods." + *period + ".timeInVeryHighMinutes",
		"timeInVeryHighMinutesDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInVeryHighMinutesDelta",

		"hasTimeInExtremeHighPercent":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInExtremeHighPercent",
		"timeInExtremeHighPercent":      "summary." + *typ + "Stats.periods." + *period + ".timeInExtremeHighPercent",
		"timeInExtremeHighPercentDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInExtremeHighPercentDelta",

		"hasTimeInExtremeHighRecords":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInExtremeHighRecords",
		"timeInExtremeHighRecords":      "summary." + *typ + "Stats.periods." + *period + ".timeInExtremeHighRecords",
		"timeInExtremeHighRecordsDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInExtremeHighRecordsDelta",

		"hasTimeInExtremeHighMinutes":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInExtremeHighMinutes",
		"timeInExtremeHighMinutes":      "summary." + *typ + "Stats.periods." + *period + ".timeInExtremeHighMinutes",
		"timeInExtremeHighMinutesDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInExtremeHighMinutesDelta",

		"hasTimeInAnyHighPercent":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInAnyHighPercent",
		"timeInAnyHighPercent":      "summary." + *typ + "Stats.periods." + *period + ".timeInAnyHighPercent",
		"timeInAnyHighPercentDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInAnyHighPercentDelta",

		"hasTimeInAnyHighRecords":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInAnyHighRecords",
		"timeInAnyHighRecords":      "summary." + *typ + "Stats.periods." + *period + ".timeInAnyHighRecords",
		"timeInAnyHighRecordsDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInAnyHighRecordsDelta",

		"hasTimeInAnyHighMinutes":   "summary." + *typ + "Stats.periods." + *period + ".hasTimeInAnyHighMinutes",
		"timeInAnyHighMinutes":      "summary." + *typ + "Stats.periods." + *period + ".timeInAnyHighMinutes",
		"timeInAnyHighMinutesDelta": "summary." + *typ + "Stats.periods." + *period + ".timeInAnyHighMinutesDelta",

		"hasAverageDailyRecords":   "summary." + *typ + "Stats.periods." + *period + ".hasAverageDailyRecords",
		"averageDailyRecords":      "summary." + *typ + "Stats.periods." + *period + ".averageDailyRecords",
		"averageDailyRecordsDelta": "summary." + *typ + "Stats.periods." + *period + ".averageDailyRecordsDelta",

		"hasTotalRecords":   "summary." + *typ + "Stats.periods." + *period + ".hasTotalRecords",
		"totalRecords":      "summary." + *typ + "Stats.periods." + *period + ".totalRecords",
		"totalRecordsDelta": "summary." + *typ + "Stats.periods." + *period + ".totalRecordsDelta",

		"hasHoursWithData":   "summary." + *typ + "Stats.periods." + *period + ".hasHoursWithData",
		"hoursWithData":      "summary." + *typ + "Stats.periods." + *period + ".hoursWithData",
		"hoursWithDataDelta": "summary." + *typ + "Stats.periods." + *period + ".hoursWithDataDelta",

		"hasDaysWithData":   "summary." + *typ + "Stats.periods." + *period + ".hasDaysWithData",
		"daysWithData":      "summary." + *typ + "Stats.periods." + *period + ".daysWithData",
		"daysWithDataDelta": "summary." + *typ + "Stats.periods." + *period + ".daysWithDataDelta",

		"hasStandardDeviation":   "summary." + *typ + "Stats.periods." + *period + ".hasStandardDeviation",
		"standardDeviation":      "summary." + *typ + "Stats.periods." + *period + ".standardDeviation",
		"standardDeviationDelta": "summary." + *typ + "Stats.periods." + *period + ".standardDeviationDelta",

		"hasCoefficientOfVariation":   "summary." + *typ + "Stats.periods." + *period + ".hasCoefficientOfVariation",
		"coefficientOfVariation":      "summary." + *typ + "Stats.periods." + *period + ".coefficientOfVariation",
		"coefficientOfVariationDelta": "summary." + *typ + "Stats.periods." + *period + ".coefficientOfVariationDelta",
	}

	extraSort := map[string]string{
		expandedSorts["lastUploadDate"]: expandedSorts["hasLastUploadDate"],
		expandedSorts["lastData"]:       expandedSorts["hasLastData"],
		expandedSorts["firstData"]:      expandedSorts["hasFirstData"],
		expandedSorts["outdatedSince"]:  expandedSorts["hasOutdatedSince"],

		expandedSorts["glucoseManagementIndicator"]:      expandedSorts["hasGlucoseManagementIndicator"],
		expandedSorts["glucoseManagementIndicatorDelta"]: expandedSorts["hasGlucoseManagementIndicatorDelta"],

		expandedSorts["averageGlucoseMmol"]:      expandedSorts["hasAverageGlucoseMmol"],
		expandedSorts["averageGlucoseMmolDelta"]: expandedSorts["hasAverageGlucoseMmolDelta"],

		expandedSorts["totalRecords"]:      expandedSorts["hasTotalRecords"],
		expandedSorts["totalRecordsDelta"]: expandedSorts["hasTotalRecordsDelta"],

		expandedSorts["hoursWithData"]:      expandedSorts["hasHoursWithData"],
		expandedSorts["hoursWithDataDelta"]: expandedSorts["hasHoursWithDataDelta"],

		expandedSorts["daysWithData"]:      expandedSorts["hasDaysWithData"],
		expandedSorts["daysWithDataDelta"]: expandedSorts["hasDaysWithDataDelta"],

		expandedSorts["standardDeviation"]:      expandedSorts["hasStandardDeviation"],
		expandedSorts["standardDeviationDelta"]: expandedSorts["hasStandardDeviationDelta"],

		expandedSorts["coefficientOfVariation"]:      expandedSorts["hasCoefficientOfVariation"],
		expandedSorts["coefficientOfVariationDelta"]: expandedSorts["hasCoefficientOfVariationDelta"],

		expandedSorts["averageDailyRecords"]:      expandedSorts["hasAverageDailyRecords"],
		expandedSorts["averageDailyRecordsDelta"]: expandedSorts["hasAverageDailyRecordsDelta"],

		expandedSorts["timeCGMUsePercent"]:      expandedSorts["hasTimeCGMUsePercent"],
		expandedSorts["timeCGMUsePercentDelta"]: expandedSorts["hasTimeCGMUsePercentDelta"],
		expandedSorts["timeCGMUseRecords"]:      expandedSorts["hasTimeCGMUseRecords"],
		expandedSorts["timeCGMUseRecordsDelta"]: expandedSorts["hasTimeCGMUseRecordsDelta"],
		expandedSorts["timeCGMUseMinutes"]:      expandedSorts["hasTimeCGMUseMinutes"],
		expandedSorts["timeCGMUseMinutesDelta"]: expandedSorts["hasTimeCGMUseMinutesDelta"],

		expandedSorts["timeInTargetPercent"]:      expandedSorts["hasTimeInTargetPercent"],
		expandedSorts["timeInTargetPercentDelta"]: expandedSorts["hasTimeInTargetPercentDelta"],
		expandedSorts["timeInTargetRecords"]:      expandedSorts["hasTimeInTargetRecords"],
		expandedSorts["timeInTargetRecordsDelta"]: expandedSorts["hasTimeInTargetRecordsDelta"],
		expandedSorts["timeInTargetMinutes"]:      expandedSorts["hasTimeInTargetMinutes"],
		expandedSorts["timeInTargetMinutesDelta"]: expandedSorts["hasTimeInTargetMinutesDelta"],

		expandedSorts["timeInLowPercent"]:      expandedSorts["hasTimeInLowPercent"],
		expandedSorts["timeInLowPercentDelta"]: expandedSorts["hasTimeInLowPercentDelta"],
		expandedSorts["timeInLowRecords"]:      expandedSorts["hasTimeInLowRecords"],
		expandedSorts["timeInLowRecordsDelta"]: expandedSorts["hasTimeInLowRecordsDelta"],
		expandedSorts["timeInLowMinutes"]:      expandedSorts["hasTimeInLowMinutes"],
		expandedSorts["timeInLowMinutesDelta"]: expandedSorts["hasTimeInLowMinutesDelta"],

		expandedSorts["timeInVeryLowPercent"]:      expandedSorts["hasTimeInVeryLowPercent"],
		expandedSorts["timeInVeryLowPercentDelta"]: expandedSorts["hasTimeInVeryLowPercentDelta"],
		expandedSorts["timeInVeryLowRecords"]:      expandedSorts["hasTimeInVeryLowRecords"],
		expandedSorts["timeInVeryLowRecordsDelta"]: expandedSorts["hasTimeInVeryLowRecordsDelta"],
		expandedSorts["timeInVeryLowMinutes"]:      expandedSorts["hasTimeInVeryLowMinutes"],
		expandedSorts["timeInVeryLowMinutesDelta"]: expandedSorts["hasTimeInVeryLowMinutesDelta"],

		expandedSorts["timeInAnyLowPercent"]:      expandedSorts["hasTimeInAnyLowPercent"],
		expandedSorts["timeInAnyLowPercentDelta"]: expandedSorts["hasTimeInAnyLowPercentDelta"],
		expandedSorts["timeInAnyLowRecords"]:      expandedSorts["hasTimeInAnyLowRecords"],
		expandedSorts["timeInAnyLowRecordsDelta"]: expandedSorts["hasTimeInAnyLowRecordsDelta"],
		expandedSorts["timeInAnyLowMinutes"]:      expandedSorts["hasTimeInAnyLowMinutes"],
		expandedSorts["timeInAnyLowMinutesDelta"]: expandedSorts["hasTimeInAnyLowMinutesDelta"],

		expandedSorts["timeInHighPercent"]:      expandedSorts["hasTimeInHighPercent"],
		expandedSorts["timeInHighPercentDelta"]: expandedSorts["hasTimeInHighPercentDelta"],
		expandedSorts["timeInHighRecords"]:      expandedSorts["hasTimeInHighRecords"],
		expandedSorts["timeInHighRecordsDelta"]: expandedSorts["hasTimeInHighRecordsDelta"],
		expandedSorts["timeInHighMinutes"]:      expandedSorts["hasTimeInHighMinutes"],
		expandedSorts["timeInHighMinutesDelta"]: expandedSorts["hasTimeInHighMinutesDelta"],

		expandedSorts["timeInVeryHighPercent"]:      expandedSorts["hasTimeInVeryHighPercent"],
		expandedSorts["timeInVeryHighPercentDelta"]: expandedSorts["hasTimeInVeryHighPercentDelta"],
		expandedSorts["timeInVeryHighRecords"]:      expandedSorts["hasTimeInVeryHighRecords"],
		expandedSorts["timeInVeryHighRecordsDelta"]: expandedSorts["hasTimeInVeryHighRecordsDelta"],
		expandedSorts["timeInVeryHighMinutes"]:      expandedSorts["hasTimeInVeryHighMinutes"],
		expandedSorts["timeInVeryHighMinutesDelta"]: expandedSorts["hasTimeInVeryHighMinutesDelta"],

		expandedSorts["timeInExtremeHighPercent"]:      expandedSorts["hasTimeInExtremeHighPercent"],
		expandedSorts["timeInExtremeHighPercentDelta"]: expandedSorts["hasTimeInExtremeHighPercentDelta"],
		expandedSorts["timeInExtremeHighRecords"]:      expandedSorts["hasTimeInExtremeHighRecords"],
		expandedSorts["timeInExtremeHighRecordsDelta"]: expandedSorts["hasTimeInExtremeHighRecordsDelta"],
		expandedSorts["timeInExtremeHighMinutes"]:      expandedSorts["hasTimeInExtremeHighMinutes"],
		expandedSorts["timeInExtremeHighMinutesDelta"]: expandedSorts["hasTimeInExtremeHighMinutesDelta"],

		expandedSorts["timeInAnyHighPercent"]:      expandedSorts["hasTimeInAnyHighPercent"],
		expandedSorts["timeInAnyHighPercentDelta"]: expandedSorts["hasTimeInAnyHighPercentDelta"],
		expandedSorts["timeInAnyHighRecords"]:      expandedSorts["hasTimeInAnyHighRecords"],
		expandedSorts["timeInAnyHighRecordsDelta"]: expandedSorts["hasTimeInAnyHighRecordsDelta"],
		expandedSorts["timeInAnyHighMinutes"]:      expandedSorts["hasTimeInAnyHighMinutes"],
		expandedSorts["timeInAnyHighMinutesDelta"]: expandedSorts["hasTimeInAnyHighMinutesDelta"],
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
		"lastReviewed":   {},
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

		"timeInExtremeHighPercent":      {},
		"timeInExtremeHighPercentDelta": {},
		"timeInExtremeHighRecords":      {},
		"timeInExtremeHighRecordsDelta": {},
		"timeInExtremeHighMinutes":      {},
		"timeInExtremeHighMinutesDelta": {},

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

		"hoursWithData":      {},
		"hoursWithDataDelta": {},

		"daysWithData":      {},
		"daysWithDataDelta": {},

		"standardDeviation":      {},
		"standardDeviationDelta": {},

		"coefficientOfVariation":      {},
		"coefficientOfVariationDelta": {},

		"averageDailyRecords":      {},
		"averageDailyRecordsDelta": {},

		"min":      {},
		"minDelta": {},

		"max":      {},
		"maxDelta": {},
	},
	"bgm": {
		"fullName":       {},
		"birthDate":      {},
		"lastUploadDate": {},
		"lastReviewed":   {},
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

		"timeInExtremeHighPercent":      {},
		"timeInExtremeHighPercentDelta": {},
		"timeInExtremeHighRecords":      {},
		"timeInExtremeHighRecordsDelta": {},

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

		"standardDeviation":      {},
		"standardDeviationDelta": {},

		"coefficientOfVariation":      {},
		"coefficientOfVariationDelta": {},

		"averageDailyRecords":      {},
		"averageDailyRecordsDelta": {},

		"min":      {},
		"minDelta": {},

		"max":      {},
		"maxDelta": {},
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

func strpuseridp(s *string) *Tidepooluserid {
	if s == nil {
		return nil
	}
	id := Tidepooluserid(*s)
	return &id
}

func useridpstrp(u *Tidepooluserid) *string {
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

func clinicSizeToString(c *ClinicV1ClinicSize) *string {
	if c == nil {
		return nil
	}
	return strp(string(*c))
}

func stringToClinicSize(s *string) *ClinicV1ClinicSize {
	if s == nil {
		return nil
	}
	size := ClinicV1ClinicSize(*s)
	return &size
}

func clinicTypeToString(c *ClinicV1ClinicType) *string {
	if c == nil {
		return nil
	}
	return strp(string(*c))
}

func stringToClinicType(s *string) *ClinicV1ClinicType {
	if s == nil {
		return nil
	}
	size := ClinicV1ClinicType(*s)
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
		"min":      params.CgmMin,
		"minDelta": params.CgmMinDelta,

		"max":      params.CgmMax,
		"maxDelta": params.CgmMaxDelta,

		"averageGlucoseMmol":         params.CgmAverageGlucoseMmol,
		"glucoseManagementIndicator": params.CgmGlucoseManagementIndicator,

		"timeCGMUsePercent":        params.CgmTimeCGMUsePercent,
		"timeInVeryLowPercent":     params.CgmTimeInVeryLowPercent,
		"timeInAnyLowPercent":      params.CgmTimeInAnyLowPercent,
		"timeInLowPercent":         params.CgmTimeInLowPercent,
		"timeInTargetPercent":      params.CgmTimeInTargetPercent,
		"timeInHighPercent":        params.CgmTimeInHighPercent,
		"timeInVeryHighPercent":    params.CgmTimeInVeryHighPercent,
		"timeInExtremeHighPercent": params.CgmTimeInExtremeHighPercent,
		"timeInAnyHighPercent":     params.CgmTimeInAnyHighPercent,

		"timeCGMUseRecords":        params.CgmTimeCGMUseRecords,
		"timeInVeryLowRecords":     params.CgmTimeInVeryLowRecords,
		"timeInAnyLowRecords":      params.CgmTimeInAnyLowRecords,
		"timeInLowRecords":         params.CgmTimeInLowRecords,
		"timeInTargetRecords":      params.CgmTimeInTargetRecords,
		"timeInHighRecords":        params.CgmTimeInHighRecords,
		"timeInVeryHighRecords":    params.CgmTimeInVeryHighRecords,
		"timeInExtremeHighRecords": params.CgmTimeInVeryHighRecords,
		"timeInAnyHighRecords":     params.CgmTimeInAnyHighRecords,

		"timeCGMUseMinutes":        params.CgmTimeCGMUseMinutes,
		"timeInVeryLowMinutes":     params.CgmTimeInVeryLowMinutes,
		"timeInAnyLowMinutes":      params.CgmTimeInAnyLowMinutes,
		"timeInLowMinutes":         params.CgmTimeInLowMinutes,
		"timeInTargetMinutes":      params.CgmTimeInTargetMinutes,
		"timeInHighMinutes":        params.CgmTimeInHighMinutes,
		"timeInVeryHighMinutes":    params.CgmTimeInVeryHighMinutes,
		"timeInExtremeHighMinutes": params.CgmTimeInVeryHighMinutes,
		"timeInAnyHighMinutes":     params.CgmTimeInAnyHighMinutes,

		"averageDailyRecords":    params.CgmAverageDailyRecords,
		"totalRecords":           params.CgmTotalRecords,
		"hoursWithData":          params.CgmHoursWithData,
		"daysWithData":           params.CgmDaysWithData,
		"standardDeviation":      params.CgmStandardDeviation,
		"coefficientOfVariation": params.CgmCoefficientOfVariation,

		"averageGlucoseMmolDelta":         params.CgmAverageGlucoseMmolDelta,
		"glucoseManagementIndicatorDelta": params.CgmGlucoseManagementIndicatorDelta,

		"timeCGMUsePercentDelta":        params.CgmTimeCGMUsePercentDelta,
		"timeInVeryLowPercentDelta":     params.CgmTimeInVeryLowPercentDelta,
		"timeInAnyLowPercentDelta":      params.CgmTimeInAnyLowPercentDelta,
		"timeInLowPercentDelta":         params.CgmTimeInLowPercentDelta,
		"timeInTargetPercentDelta":      params.CgmTimeInTargetPercentDelta,
		"timeInHighPercentDelta":        params.CgmTimeInHighPercentDelta,
		"timeInVeryHighPercentDelta":    params.CgmTimeInVeryHighPercentDelta,
		"timeInExtremeHighPercentDelta": params.CgmTimeInExtremeHighPercentDelta,
		"timeInAnyHighPercentDelta":     params.CgmTimeInAnyHighPercentDelta,

		"timeCGMUseRecordsDelta":        params.CgmTimeCGMUseRecordsDelta,
		"timeInVeryLowRecordsDelta":     params.CgmTimeInVeryLowRecordsDelta,
		"timeInAnyLowRecordsDelta":      params.CgmTimeInAnyLowRecordsDelta,
		"timeInLowRecordsDelta":         params.CgmTimeInLowRecordsDelta,
		"timeInTargetRecordsDelta":      params.CgmTimeInTargetRecordsDelta,
		"timeInHighRecordsDelta":        params.CgmTimeInHighRecordsDelta,
		"timeInVeryHighRecordsDelta":    params.CgmTimeInVeryHighRecordsDelta,
		"timeInExtremeHighRecordsDelta": params.CgmTimeInVeryHighRecordsDelta,
		"timeInAnyHighRecordsDelta":     params.CgmTimeInAnyHighRecordsDelta,

		"timeCGMUseMinutesDelta":        params.CgmTimeCGMUseMinutesDelta,
		"timeInVeryLowMinutesDelta":     params.CgmTimeInVeryLowMinutesDelta,
		"timeInAnyLowMinutesDelta":      params.CgmTimeInAnyLowMinutesDelta,
		"timeInLowMinutesDelta":         params.CgmTimeInLowMinutesDelta,
		"timeInTargetMinutesDelta":      params.CgmTimeInTargetMinutesDelta,
		"timeInHighMinutesDelta":        params.CgmTimeInHighMinutesDelta,
		"timeInVeryHighMinutesDelta":    params.CgmTimeInVeryHighMinutesDelta,
		"timeInExtremeHighMinutesDelta": params.CgmTimeInVeryHighMinutesDelta,
		"timeInAnyHighMinutesDelta":     params.CgmTimeInAnyHighMinutesDelta,

		"averageDailyRecordsDelta":    params.CgmAverageDailyRecordsDelta,
		"totalRecordsDelta":           params.CgmTotalRecordsDelta,
		"hoursWithDataDelta":          params.CgmHoursWithDataDelta,
		"daysWithDataDelta":           params.CgmDaysWithDataDelta,
		"standardDeviationDelta":      params.CgmStandardDeviationDelta,
		"coefficientOfVariationDelta": params.CgmCoefficientOfVariationDelta,
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
		"max": params.BgmMax,
		"min": params.BgmMin,

		"averageGlucoseMmol": params.BgmAverageGlucoseMmol,

		"timeInVeryLowPercent":   params.BgmTimeInVeryLowPercent,
		"timeInAnyLowPercent":    params.BgmTimeInAnyLowPercent,
		"timeInLowPercent":       params.BgmTimeInLowPercent,
		"timeInTargetPercent":    params.BgmTimeInTargetPercent,
		"timeInHighPercent":      params.BgmTimeInHighPercent,
		"timeInVeryHighPercent":  params.BgmTimeInVeryHighPercent,
		"timeExtremeHighPercent": params.BgmTimeInVeryHighPercent,
		"timeInAnyHighPercent":   params.BgmTimeInAnyHighPercent,

		"timeInVeryLowRecords":     params.BgmTimeInVeryLowRecords,
		"timeInAnyLowRecords":      params.BgmTimeInAnyLowRecords,
		"timeInLowRecords":         params.BgmTimeInLowRecords,
		"timeInTargetRecords":      params.BgmTimeInTargetRecords,
		"timeInHighRecords":        params.BgmTimeInHighRecords,
		"timeInVeryHighRecords":    params.BgmTimeInVeryHighRecords,
		"timeInExtremeHighRecords": params.BgmTimeInVeryHighRecords,
		"timeInAnyHighRecords":     params.BgmTimeInAnyHighRecords,

		"averageDailyRecords": params.BgmAverageDailyRecords,
		"totalRecords":        params.BgmTotalRecords,

		"averageGlucoseMmolDelta": params.BgmAverageGlucoseMmolDelta,

		"timeInVeryLowPercentDelta":   params.BgmTimeInVeryLowPercentDelta,
		"timeInAnyLowPercentDelta":    params.BgmTimeInAnyLowPercentDelta,
		"timeInLowPercentDelta":       params.BgmTimeInLowPercentDelta,
		"timeInTargetPercentDelta":    params.BgmTimeInTargetPercentDelta,
		"timeInHighPercentDelta":      params.BgmTimeInHighPercentDelta,
		"timeInVeryHighPercentDelta":  params.BgmTimeInVeryHighPercentDelta,
		"timeExtremeHighPercentDelta": params.BgmTimeInVeryHighPercentDelta,
		"timeInAnyHighPercentDelta":   params.BgmTimeInAnyHighPercentDelta,

		"timeInVeryLowRecordsDelta":     params.BgmTimeInVeryLowRecordsDelta,
		"timeInAnyLowRecordsDelta":      params.BgmTimeInAnyLowRecordsDelta,
		"timeInLowRecordsDelta":         params.BgmTimeInLowRecordsDelta,
		"timeInTargetRecordsDelta":      params.BgmTimeInTargetRecordsDelta,
		"timeInHighRecordsDelta":        params.BgmTimeInHighRecordsDelta,
		"timeInVeryHighRecordsDelta":    params.BgmTimeInVeryHighRecordsDelta,
		"timeInExtremeHighRecordsDelta": params.BgmTimeInVeryHighRecordsDelta,
		"timeInAnyHighRecordsDelta":     params.BgmTimeInAnyHighRecordsDelta,

		"averageDailyRecordsDelta": params.BgmAverageDailyRecordsDelta,
		"totalRecordsDelta":        params.BgmTotalRecordsDelta,
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

	parseDateRangeFilter(filters, "lastData", params.CgmLastDataFrom, params.CgmLastDataTo)
	return
}

func ParseBGMSummaryDateFilters(params ListPatientsParams) (filters patients.SummaryDateFilters) {
	filters = patients.SummaryDateFilters{}

	parseDateRangeFilter(filters, "lastData", params.BgmLastDataFrom, params.BgmLastDataTo)
	return
}

func NewDataProvider(providerId ProviderId) (string, error) {
	switch providerId {
	case Dexcom:
		return patients.DexcomDataSourceProviderName, nil
	case Twiist:
		return patients.TwiistDataSourceProviderName, nil
	case Abbott:
		return patients.AbbottDataSourceProviderName, nil
	default:
		return "", fmt.Errorf("%w: invalid provider id: %s", errors.BadRequest, string(providerId))
	}
}

func NewMatchOrderCriteria(criteria []EhrMatchRequestPatientsOptionsV1Criteria) ([]string, error) {
	result := make([]string, 0, len(criteria))
	for _, c := range criteria {
		val := string(c)
		result = append(result, val)
	}

	return result, nil
}
