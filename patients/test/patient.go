package test

import (
	"math/rand"
	"time"

	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var permissions = []string{"view", "upload", "note", "custodian"}

func strp(s string) *string {
	return &s
}

func RandomPatient() patients.Patient {
	clinicId := primitive.NewObjectID()
	devices := []string{test.Faker.Company().Name(), test.Faker.Company().Name(), test.Faker.Company().Name()}
	tags := []primitive.ObjectID{primitive.NewObjectID()}
	permissions := RandomPermissions()
	dataSources := RandomDataSources()
	return patients.Patient{
		ClinicId:      &clinicId,
		UserId:        strp(test.Faker.UUID().V4()),
		BirthDate:     strp(test.Faker.Time().ISO8601(time.Now())[:10]),
		Email:         strp(test.Faker.Internet().Email()),
		FullName:      strp(test.Faker.Person().Name()),
		Mrn:           strp(test.Faker.UUID().V4()),
		Tags:          &tags,
		TargetDevices: &devices,
		Permissions:   &permissions,
		IsMigrated:    test.Faker.Bool(),
		DataSources:   (*[]patients.DataSource)(&dataSources),
	}
}

func RandomPatientUpdate() patients.PatientUpdate {
	patient := RandomPatient()
	return patients.PatientUpdate{
		Patient: patients.Patient{
			BirthDate:     patient.BirthDate,
			Email:         patient.Email,
			FullName:      patient.FullName,
			Mrn:           patient.Mrn,
			Tags:          patient.Tags,
			TargetDevices: patient.TargetDevices,
			Permissions:   patient.Permissions,
			DataSources:   patient.DataSources,
		},
	}
}

func RandomPermission() string {
	return test.Faker.RandomStringElement(permissions)
}

func RandomPermissions() patients.Permissions {
	a := append([]string{}, permissions...)
	rand.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
	count := test.Faker.IntBetween(1, len(a))
	a = a[:count]
	permissions := patients.Permissions{}
	for _, p := range a {
		setPermission(&permissions, p)
	}
	return permissions
}

func RandomDataSources() patients.DataSources {
	return []patients.DataSource{
		{State: test.Faker.RandomStringElement([]string{"pending", "connected"}), ProviderName: test.Faker.Company().Name()},
	}
}

func setPermission(permissions *patients.Permissions, p string) {
	switch p {
	case "view":
		permissions.View = &patients.Permission{}
	case "note":
		permissions.View = &patients.Permission{}
	case "upload":
		permissions.Upload = &patients.Permission{}
	case "custodian":
		permissions.Custodian = &patients.Permission{}
	}
}

func ptr[T any](v T) *T {
	return &v
}

func RandomSummary() (userSummary patients.Summary) {
	userSummary = patients.Summary{
		CGM: &patients.PatientCGMStats{
			Periods:       &patients.PatientCGMPeriods{},
			OffsetPeriods: nil,
			Dates:         nil,
			Config:        nil,
			TotalHours:    nil,
		},
		BGM: &patients.PatientBGMStats{
			Periods:       &patients.PatientBGMPeriods{},
			OffsetPeriods: nil,
			Dates:         nil,
			Config:        nil,
			TotalHours:    nil,
		},
	}

	for _, periodKey := range []string{"1d", "7d", "14d", "30d"} {
		(*userSummary.CGM.Periods)[periodKey] = patients.PatientCGMPeriod{
			AverageDailyRecords:             ptr(test.Faker.Float64(4, 0, 20)),
			AverageDailyRecordsDelta:        ptr(test.Faker.Float64(4, -20, 20)),
			AverageGlucoseMmol:              ptr(test.Faker.Float64(4, 4, 20)),
			AverageGlucoseMmolDelta:         ptr(test.Faker.Float64(4, -10, 10)),
			GlucoseManagementIndicator:      ptr(test.Faker.Float64(4, 4, 20)),
			GlucoseManagementIndicatorDelta: ptr(test.Faker.Float64(4, -10, 10)),
			HasAverageDailyRecords:          ptr(test.Faker.Bool()),
			HasAverageGlucoseMmol:           ptr(test.Faker.Bool()),
			HasGlucoseManagementIndicator:   ptr(test.Faker.Bool()),
			HasTimeCGMUseMinutes:            ptr(test.Faker.Bool()),
			HasTimeCGMUsePercent:            ptr(test.Faker.Bool()),
			HasTimeCGMUseRecords:            ptr(test.Faker.Bool()),
			HasTimeInHighMinutes:            ptr(test.Faker.Bool()),
			HasTimeInHighPercent:            ptr(test.Faker.Bool()),
			HasTimeInHighRecords:            ptr(test.Faker.Bool()),
			HasTimeInLowMinutes:             ptr(test.Faker.Bool()),
			HasTimeInLowPercent:             ptr(test.Faker.Bool()),
			HasTimeInLowRecords:             ptr(test.Faker.Bool()),
			HasTimeInTargetMinutes:          ptr(test.Faker.Bool()),
			HasTimeInTargetPercent:          ptr(test.Faker.Bool()),
			HasTimeInTargetRecords:          ptr(test.Faker.Bool()),
			HasTimeInVeryHighMinutes:        ptr(test.Faker.Bool()),
			HasTimeInVeryHighPercent:        ptr(test.Faker.Bool()),
			HasTimeInVeryHighRecords:        ptr(test.Faker.Bool()),
			HasTimeInVeryLowMinutes:         ptr(test.Faker.Bool()),
			HasTimeInVeryLowPercent:         ptr(test.Faker.Bool()),
			HasTimeInVeryLowRecords:         ptr(test.Faker.Bool()),
			HasTotalRecords:                 ptr(test.Faker.Bool()),
			TimeCGMUseMinutes:               ptr(test.Faker.IntBetween(0, 2000)),
			TimeCGMUseMinutesDelta:          ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeCGMUsePercent:               ptr(test.Faker.Float64(4, 0, 1)),
			TimeCGMUsePercentDelta:          ptr(test.Faker.Float64(4, -1, 1)),
			TimeCGMUseRecords:               ptr(test.Faker.IntBetween(0, 300)),
			TimeCGMUseRecordsDelta:          ptr(test.Faker.IntBetween(-300, 300)),
			TimeInHighMinutes:               ptr(test.Faker.IntBetween(0, 2000)),
			TimeInHighMinutesDelta:          ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeInHighPercent:               ptr(test.Faker.Float64(4, 0, 1)),
			TimeInHighPercentDelta:          ptr(test.Faker.Float64(4, -1, 1)),
			TimeInHighRecords:               ptr(test.Faker.IntBetween(0, 300)),
			TimeInHighRecordsDelta:          ptr(test.Faker.IntBetween(-300, 300)),
			TimeInLowMinutes:                ptr(test.Faker.IntBetween(0, 2000)),
			TimeInLowMinutesDelta:           ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeInLowPercent:                ptr(test.Faker.Float64(4, 0, 1)),
			TimeInLowPercentDelta:           ptr(test.Faker.Float64(4, -1, 1)),
			TimeInLowRecords:                ptr(test.Faker.IntBetween(0, 300)),
			TimeInLowRecordsDelta:           ptr(test.Faker.IntBetween(-300, 300)),
			TimeInTargetMinutes:             ptr(test.Faker.IntBetween(0, 2000)),
			TimeInTargetMinutesDelta:        ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeInTargetPercent:             ptr(test.Faker.Float64(4, 0, 1)),
			TimeInTargetPercentDelta:        ptr(test.Faker.Float64(4, -1, 1)),
			TimeInTargetRecords:             ptr(test.Faker.IntBetween(0, 300)),
			TimeInTargetRecordsDelta:        ptr(test.Faker.IntBetween(-300, 300)),
			TimeInVeryHighMinutes:           ptr(test.Faker.IntBetween(0, 2000)),
			TimeInVeryHighMinutesDelta:      ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeInVeryHighPercent:           ptr(test.Faker.Float64(4, 0, 1)),
			TimeInVeryHighPercentDelta:      ptr(test.Faker.Float64(4, -1, 1)),
			TimeInVeryHighRecords:           ptr(test.Faker.IntBetween(0, 300)),
			TimeInVeryHighRecordsDelta:      ptr(test.Faker.IntBetween(-300, 300)),
			TimeInVeryLowMinutes:            ptr(test.Faker.IntBetween(0, 2000)),
			TimeInVeryLowMinutesDelta:       ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeInVeryLowPercent:            ptr(test.Faker.Float64(4, 0, 1)),
			TimeInVeryLowPercentDelta:       ptr(test.Faker.Float64(4, -1, 1)),
			TimeInVeryLowRecords:            ptr(test.Faker.IntBetween(0, 300)),
			TimeInVeryLowRecordsDelta:       ptr(test.Faker.IntBetween(-300, 300)),
			TotalRecords:                    ptr(test.Faker.IntBetween(0, 300)),
			TotalRecordsDelta:               ptr(test.Faker.IntBetween(-300, 300)),
		}

		(*userSummary.CGM.OffsetPeriods)[periodKey] = patients.PatientCGMPeriod{
			AverageDailyRecords:             ptr(test.Faker.Float64(4, 0, 20)),
			AverageDailyRecordsDelta:        ptr(test.Faker.Float64(4, -20, 20)),
			AverageGlucoseMmol:              ptr(test.Faker.Float64(4, 4, 20)),
			AverageGlucoseMmolDelta:         ptr(test.Faker.Float64(4, -10, 10)),
			GlucoseManagementIndicator:      ptr(test.Faker.Float64(4, 4, 20)),
			GlucoseManagementIndicatorDelta: ptr(test.Faker.Float64(4, -10, 10)),
			HasAverageDailyRecords:          ptr(test.Faker.Bool()),
			HasAverageGlucoseMmol:           ptr(test.Faker.Bool()),
			HasGlucoseManagementIndicator:   ptr(test.Faker.Bool()),
			HasTimeCGMUseMinutes:            ptr(test.Faker.Bool()),
			HasTimeCGMUsePercent:            ptr(test.Faker.Bool()),
			HasTimeCGMUseRecords:            ptr(test.Faker.Bool()),
			HasTimeInHighMinutes:            ptr(test.Faker.Bool()),
			HasTimeInHighPercent:            ptr(test.Faker.Bool()),
			HasTimeInHighRecords:            ptr(test.Faker.Bool()),
			HasTimeInLowMinutes:             ptr(test.Faker.Bool()),
			HasTimeInLowPercent:             ptr(test.Faker.Bool()),
			HasTimeInLowRecords:             ptr(test.Faker.Bool()),
			HasTimeInTargetMinutes:          ptr(test.Faker.Bool()),
			HasTimeInTargetPercent:          ptr(test.Faker.Bool()),
			HasTimeInTargetRecords:          ptr(test.Faker.Bool()),
			HasTimeInVeryHighMinutes:        ptr(test.Faker.Bool()),
			HasTimeInVeryHighPercent:        ptr(test.Faker.Bool()),
			HasTimeInVeryHighRecords:        ptr(test.Faker.Bool()),
			HasTimeInVeryLowMinutes:         ptr(test.Faker.Bool()),
			HasTimeInVeryLowPercent:         ptr(test.Faker.Bool()),
			HasTimeInVeryLowRecords:         ptr(test.Faker.Bool()),
			HasTotalRecords:                 ptr(test.Faker.Bool()),
			TimeCGMUseMinutes:               ptr(test.Faker.IntBetween(0, 2000)),
			TimeCGMUseMinutesDelta:          ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeCGMUsePercent:               ptr(test.Faker.Float64(4, 0, 1)),
			TimeCGMUsePercentDelta:          ptr(test.Faker.Float64(4, -1, 1)),
			TimeCGMUseRecords:               ptr(test.Faker.IntBetween(0, 300)),
			TimeCGMUseRecordsDelta:          ptr(test.Faker.IntBetween(-300, 300)),
			TimeInHighMinutes:               ptr(test.Faker.IntBetween(0, 2000)),
			TimeInHighMinutesDelta:          ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeInHighPercent:               ptr(test.Faker.Float64(4, 0, 1)),
			TimeInHighPercentDelta:          ptr(test.Faker.Float64(4, -1, 1)),
			TimeInHighRecords:               ptr(test.Faker.IntBetween(0, 300)),
			TimeInHighRecordsDelta:          ptr(test.Faker.IntBetween(-300, 300)),
			TimeInLowMinutes:                ptr(test.Faker.IntBetween(0, 2000)),
			TimeInLowMinutesDelta:           ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeInLowPercent:                ptr(test.Faker.Float64(4, 0, 1)),
			TimeInLowPercentDelta:           ptr(test.Faker.Float64(4, -1, 1)),
			TimeInLowRecords:                ptr(test.Faker.IntBetween(0, 300)),
			TimeInLowRecordsDelta:           ptr(test.Faker.IntBetween(-300, 300)),
			TimeInTargetMinutes:             ptr(test.Faker.IntBetween(0, 2000)),
			TimeInTargetMinutesDelta:        ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeInTargetPercent:             ptr(test.Faker.Float64(4, 0, 1)),
			TimeInTargetPercentDelta:        ptr(test.Faker.Float64(4, -1, 1)),
			TimeInTargetRecords:             ptr(test.Faker.IntBetween(0, 300)),
			TimeInTargetRecordsDelta:        ptr(test.Faker.IntBetween(-300, 300)),
			TimeInVeryHighMinutes:           ptr(test.Faker.IntBetween(0, 2000)),
			TimeInVeryHighMinutesDelta:      ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeInVeryHighPercent:           ptr(test.Faker.Float64(4, 0, 1)),
			TimeInVeryHighPercentDelta:      ptr(test.Faker.Float64(4, -1, 1)),
			TimeInVeryHighRecords:           ptr(test.Faker.IntBetween(0, 300)),
			TimeInVeryHighRecordsDelta:      ptr(test.Faker.IntBetween(-300, 300)),
			TimeInVeryLowMinutes:            ptr(test.Faker.IntBetween(0, 2000)),
			TimeInVeryLowMinutesDelta:       ptr(test.Faker.IntBetween(-2000, 2000)),
			TimeInVeryLowPercent:            ptr(test.Faker.Float64(4, 0, 1)),
			TimeInVeryLowPercentDelta:       ptr(test.Faker.Float64(4, -1, 1)),
			TimeInVeryLowRecords:            ptr(test.Faker.IntBetween(0, 300)),
			TimeInVeryLowRecordsDelta:       ptr(test.Faker.IntBetween(-300, 300)),
			TotalRecords:                    ptr(test.Faker.IntBetween(0, 300)),
			TotalRecordsDelta:               ptr(test.Faker.IntBetween(-300, 300)),
		}

		(*userSummary.BGM.Periods)[periodKey] = patients.PatientBGMPeriod{
			AverageDailyRecords:        ptr(test.Faker.Float64(4, 0, 20)),
			AverageDailyRecordsDelta:   ptr(test.Faker.Float64(4, -20, 20)),
			AverageGlucoseMmol:         ptr(test.Faker.Float64(4, 4, 20)),
			AverageGlucoseMmolDelta:    ptr(test.Faker.Float64(4, -10, 10)),
			HasAverageDailyRecords:     ptr(test.Faker.Bool()),
			HasAverageGlucoseMmol:      ptr(test.Faker.Bool()),
			HasTimeInHighPercent:       ptr(test.Faker.Bool()),
			HasTimeInHighRecords:       ptr(test.Faker.Bool()),
			HasTimeInLowPercent:        ptr(test.Faker.Bool()),
			HasTimeInLowRecords:        ptr(test.Faker.Bool()),
			HasTimeInTargetPercent:     ptr(test.Faker.Bool()),
			HasTimeInTargetRecords:     ptr(test.Faker.Bool()),
			HasTimeInVeryHighPercent:   ptr(test.Faker.Bool()),
			HasTimeInVeryHighRecords:   ptr(test.Faker.Bool()),
			HasTimeInVeryLowPercent:    ptr(test.Faker.Bool()),
			HasTimeInVeryLowRecords:    ptr(test.Faker.Bool()),
			HasTotalRecords:            ptr(test.Faker.Bool()),
			TimeInHighPercent:          ptr(test.Faker.Float64(4, 0, 1)),
			TimeInHighPercentDelta:     ptr(test.Faker.Float64(4, -1, 1)),
			TimeInHighRecords:          ptr(test.Faker.IntBetween(0, 300)),
			TimeInHighRecordsDelta:     ptr(test.Faker.IntBetween(-300, 300)),
			TimeInLowPercent:           ptr(test.Faker.Float64(4, 0, 1)),
			TimeInLowPercentDelta:      ptr(test.Faker.Float64(4, -1, 1)),
			TimeInLowRecords:           ptr(test.Faker.IntBetween(0, 300)),
			TimeInLowRecordsDelta:      ptr(test.Faker.IntBetween(-300, 300)),
			TimeInTargetPercent:        ptr(test.Faker.Float64(4, 0, 1)),
			TimeInTargetPercentDelta:   ptr(test.Faker.Float64(4, -1, 1)),
			TimeInTargetRecords:        ptr(test.Faker.IntBetween(0, 300)),
			TimeInTargetRecordsDelta:   ptr(test.Faker.IntBetween(-300, 300)),
			TimeInVeryHighPercent:      ptr(test.Faker.Float64(4, 0, 1)),
			TimeInVeryHighPercentDelta: ptr(test.Faker.Float64(4, -1, 1)),
			TimeInVeryHighRecords:      ptr(test.Faker.IntBetween(0, 300)),
			TimeInVeryHighRecordsDelta: ptr(test.Faker.IntBetween(-300, 300)),
			TimeInVeryLowPercent:       ptr(test.Faker.Float64(4, 0, 1)),
			TimeInVeryLowPercentDelta:  ptr(test.Faker.Float64(4, -1, 1)),
			TimeInVeryLowRecords:       ptr(test.Faker.IntBetween(0, 300)),
			TimeInVeryLowRecordsDelta:  ptr(test.Faker.IntBetween(-300, 300)),
			TotalRecords:               ptr(test.Faker.IntBetween(0, 300)),
			TotalRecordsDelta:          ptr(test.Faker.IntBetween(-300, 300)),
		}

		(*userSummary.BGM.OffsetPeriods)[periodKey] = patients.PatientBGMPeriod{
			AverageDailyRecords:        ptr(test.Faker.Float64(4, 0, 20)),
			AverageDailyRecordsDelta:   ptr(test.Faker.Float64(4, -20, 20)),
			AverageGlucoseMmol:         ptr(test.Faker.Float64(4, 4, 20)),
			AverageGlucoseMmolDelta:    ptr(test.Faker.Float64(4, -10, 10)),
			HasAverageDailyRecords:     ptr(test.Faker.Bool()),
			HasAverageGlucoseMmol:      ptr(test.Faker.Bool()),
			HasTimeInHighPercent:       ptr(test.Faker.Bool()),
			HasTimeInHighRecords:       ptr(test.Faker.Bool()),
			HasTimeInLowPercent:        ptr(test.Faker.Bool()),
			HasTimeInLowRecords:        ptr(test.Faker.Bool()),
			HasTimeInTargetPercent:     ptr(test.Faker.Bool()),
			HasTimeInTargetRecords:     ptr(test.Faker.Bool()),
			HasTimeInVeryHighPercent:   ptr(test.Faker.Bool()),
			HasTimeInVeryHighRecords:   ptr(test.Faker.Bool()),
			HasTimeInVeryLowPercent:    ptr(test.Faker.Bool()),
			HasTimeInVeryLowRecords:    ptr(test.Faker.Bool()),
			HasTotalRecords:            ptr(test.Faker.Bool()),
			TimeInHighPercent:          ptr(test.Faker.Float64(4, 0, 1)),
			TimeInHighPercentDelta:     ptr(test.Faker.Float64(4, -1, 1)),
			TimeInHighRecords:          ptr(test.Faker.IntBetween(0, 300)),
			TimeInHighRecordsDelta:     ptr(test.Faker.IntBetween(-300, 300)),
			TimeInLowPercent:           ptr(test.Faker.Float64(4, 0, 1)),
			TimeInLowPercentDelta:      ptr(test.Faker.Float64(4, -1, 1)),
			TimeInLowRecords:           ptr(test.Faker.IntBetween(0, 300)),
			TimeInLowRecordsDelta:      ptr(test.Faker.IntBetween(-300, 300)),
			TimeInTargetPercent:        ptr(test.Faker.Float64(4, 0, 1)),
			TimeInTargetPercentDelta:   ptr(test.Faker.Float64(4, -1, 1)),
			TimeInTargetRecords:        ptr(test.Faker.IntBetween(0, 300)),
			TimeInTargetRecordsDelta:   ptr(test.Faker.IntBetween(-300, 300)),
			TimeInVeryHighPercent:      ptr(test.Faker.Float64(4, 0, 1)),
			TimeInVeryHighPercentDelta: ptr(test.Faker.Float64(4, -1, 1)),
			TimeInVeryHighRecords:      ptr(test.Faker.IntBetween(0, 300)),
			TimeInVeryHighRecordsDelta: ptr(test.Faker.IntBetween(-300, 300)),
			TimeInVeryLowPercent:       ptr(test.Faker.Float64(4, 0, 1)),
			TimeInVeryLowPercentDelta:  ptr(test.Faker.Float64(4, -1, 1)),
			TimeInVeryLowRecords:       ptr(test.Faker.IntBetween(0, 300)),
			TimeInVeryLowRecordsDelta:  ptr(test.Faker.IntBetween(-300, 300)),
			TotalRecords:               ptr(test.Faker.IntBetween(0, 300)),
			TotalRecordsDelta:          ptr(test.Faker.IntBetween(-300, 300)),
		}
	}

	return
}
