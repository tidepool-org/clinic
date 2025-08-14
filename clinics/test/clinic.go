package test

import (
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/tidepool-org/clinic/test"
	"time"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	"go.mongodb.org/mongo-driver/bson/primitive"
)



func strp(val string) *string {
	return &val
}

func RandomClinics(count int) []*clinics.Clinic {
	if count <= 0 {
		panic("count must be positive")
	}

	clinics := make([]*clinics.Clinic, count)
	for i := 0; i < count; i++ {
		clinics[i] = RandomClinic()
	}
	return clinics
}

func RandomClinic() *clinics.Clinic {
	shareCode := test.Faker.UUID().V4()
	shareCodes := []string{shareCode}
	admins := []string{test.Faker.UUID().V4()}
	id := RandomObjectId()

	clinic := clinics.NewClinicWithDefaults()
	clinic.Id = &id
	clinic.Address = strp(test.Faker.Address().Address())
	clinic.City = strp(test.Faker.Address().City())
	clinic.ClinicType = strp(test.Faker.RandomStringElement([]string{"Diabetes Clinic", "Hospital"}))
	clinic.ClinicSize = strp(test.Faker.RandomStringElement([]string{"0-100", "100-1000"}))
	clinic.Country = strp(test.Faker.Address().Country())
	clinic.Name = strp(test.Faker.Company().Name())
	clinic.PatientTags = RandomTags(3)
	clinic.PostalCode = strp(test.Faker.Address().PostCode())
	clinic.State = strp(test.Faker.Address().State())
	clinic.CanonicalShareCode = strp(shareCode)
	clinic.Website = strp(test.Faker.Internet().Domain())
	clinic.ShareCodes = &shareCodes
	clinic.Admins = &admins
	clinic.CreatedTime = test.Faker.Time().Time(time.Now())
	clinic.UpdatedTime = test.Faker.Time().Time(time.Now())
	clinic.IsMigrated = false
	clinic.EHRSettings = &clinics.EHRSettings{
		Enabled:  true,
		Provider: "xealth",
		ProcedureCodes: clinics.EHRProcedureCodes{
			EnableSummaryReports:          strp("12345"),
			DisableSummaryReports:         strp("23456"),
			CreateAccount:                 strp("34567"),
			CreateAccountAndEnableReports: strp("45678"),
		},
		SourceId: test.Faker.UUID().V4(),
	}
	return clinic
}

func RandomTags(count int) []clinics.PatientTag {
	names := mapset.NewSet[string]()
	tags := make([]clinics.PatientTag, count)
	for i, _ := range tags {
		id := RandomObjectId()
		name := fmt.Sprintf("%d %.20s", test.Faker.Company().EIN(), test.Faker.Company().Name())
		for names.Contains(name) {
			name = fmt.Sprintf("%d %.20s", test.Faker.Company().EIN(), test.Faker.Company().Name())
		}
		names.Append(name)
		tags[i].Id = &id
		tags[i].Name = name
	}

	return tags
}

func RandomObjectId() primitive.ObjectID {
	return primitive.NewObjectIDFromTimestamp(test.Faker.Time().TimeBetween(time.Now().Add(-time.Hour*24*365), time.Now()))
}

func RandomClinicCreate() *manager.CreateClinic {
	userId := test.Faker.UUID().V4()
	clinic := RandomClinic()
	clinic.Admins = nil
	clinic.CanonicalShareCode = nil
	clinic.ShareCodes = nil

	return &manager.CreateClinic{
		Clinic:            *clinic,
		CreatorUserId:     userId,
		CreateDemoPatient: false,
	}
}

func EnableXealth(clinic *clinics.Clinic, deployment string) *clinics.Clinic {
	clinic.EHRSettings = &clinics.EHRSettings{
		Enabled:  true,
		Provider: "xealth",
		ProcedureCodes: clinics.EHRProcedureCodes{
			EnableSummaryReports:          strp("12345"),
			DisableSummaryReports:         strp("23456"),
			CreateAccount:                 strp("34567"),
			CreateAccountAndEnableReports: strp("45678"),
		},
		SourceId: deployment,
	}
	return clinic
}
