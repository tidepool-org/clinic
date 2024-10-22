package test

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/jaswdr/faker"
	"github.com/onsi/ginkgo/v2"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	Faker = faker.NewWithSeed(rand.NewSource(ginkgo.GinkgoRandomSeed()))
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
	shareCode := Faker.UUID().V4()
	shareCodes := []string{shareCode}
	admins := []string{Faker.UUID().V4()}
	id := RandomObjectId()

	clinic := clinics.NewClinicWithDefaults()
	clinic.Id = &id
	clinic.Address = strp(Faker.Address().Address())
	clinic.City = strp(Faker.Address().City())
	clinic.ClinicType = strp(Faker.RandomStringElement([]string{"Diabetes Clinic", "Hospital"}))
	clinic.ClinicSize = strp(Faker.RandomStringElement([]string{"0-100", "100-1000"}))
	clinic.Country = strp(Faker.Address().Country())
	clinic.Name = strp(Faker.Company().Name())
	clinic.PatientTags = RandomTags(3)
	clinic.PostalCode = strp(Faker.Address().PostCode())
	clinic.State = strp(Faker.Address().State())
	clinic.CanonicalShareCode = strp(shareCode)
	clinic.Website = strp(Faker.Internet().Domain())
	clinic.ShareCodes = &shareCodes
	clinic.Admins = &admins
	clinic.CreatedTime = Faker.Time().Time(time.Now())
	clinic.UpdatedTime = Faker.Time().Time(time.Now())
	clinic.IsMigrated = false
	clinic.EHRSettings = &clinics.EHRSettings{
		Enabled:  true,
		Provider: "xealth",
		Facility: &clinics.EHRFacility{
			Name: Faker.Company().Name(),
		},
		ProcedureCodes: clinics.EHRProcedureCodes{
			EnableSummaryReports:          strp("12345"),
			DisableSummaryReports:         strp("23456"),
			CreateAccount:                 strp("34567"),
			CreateAccountAndEnableReports: strp("45678"),
		},
		SourceId: Faker.UUID().V4(),
	}
	return clinic
}

func RandomTags(count int) []clinics.PatientTag {
	tags := make([]clinics.PatientTag, count)
	for i, _ := range tags {
		id := RandomObjectId()
		tags[i].Id = &id
		tags[i].Name = fmt.Sprintf("%.20s", Faker.Person().LastName())
	}

	return tags
}

func RandomObjectId() primitive.ObjectID {
	return primitive.NewObjectIDFromTimestamp(Faker.Time().TimeBetween(time.Now().Add(-time.Hour*24*365), time.Now()))
}

func RandomClinicCreate() *manager.CreateClinic {
	userId := Faker.UUID().V4()
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
		Facility: &clinics.EHRFacility{
			Name: Faker.Company().Name(),
		},
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
