package test

import (
	"github.com/jaswdr/faker"
	"github.com/onsi/ginkgo/v2"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math/rand"
	"time"
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
	id := primitive.NewObjectIDFromTimestamp(Faker.Time().TimeBetween(time.Now().Add(-time.Hour*24*365), time.Now()))
	createAccountCode := "34567"

	return &clinics.Clinic{
		Id:                 &id,
		Address:            strp(Faker.Address().Address()),
		City:               strp(Faker.Address().City()),
		ClinicType:         strp(Faker.RandomStringElement([]string{"Diabetes Clinic", "Hospital"})),
		ClinicSize:         strp(Faker.RandomStringElement([]string{"0-100", "100-1000"})),
		Country:            strp(Faker.Address().Country()),
		Name:               strp(Faker.Company().Name()),
		PostalCode:         strp(Faker.Address().PostCode()),
		State:              strp(Faker.Address().State()),
		CanonicalShareCode: strp(shareCode),
		Website:            strp(Faker.Internet().Domain()),
		ShareCodes:         &shareCodes,
		Admins:             &admins,
		CreatedTime:        Faker.Time().Time(time.Now()),
		UpdatedTime:        Faker.Time().Time(time.Now()),
		IsMigrated:         false,
		EHRSettings: &clinics.EHRSettings{
			Enabled: true,
			Facility: &clinics.EHRFacility{
				Name: Faker.Company().Name(),
			},
			ProcedureCodes: clinics.EHRProcedureCodes{
				EnableSummaryReports:  "12345",
				DisableSummaryReports: "23456",
				CreateAccount:         &createAccountCode,
			},
			SourceId: Faker.UUID().V4(),
		},
	}
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
