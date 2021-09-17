package test

import (
	"github.com/jaswdr/faker"
	"github.com/onsi/ginkgo/config"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/creator"
	"math/rand"
	"time"
)

var (
	Faker = faker.NewWithSeed(rand.NewSource(config.GinkgoConfig.RandomSeed))
)

func strp(val string) *string {
	return &val
}

func RandomClinic() *clinics.Clinic {
	shareCode := Faker.UUID().V4()
	shareCodes := []string{shareCode}
	admins := []string{Faker.UUID().V4()}
	return &clinics.Clinic{
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
		CreatedAt:          Faker.Time().Time(time.Now()),
		UpdatedAt:          Faker.Time().Time(time.Now()),
		IsMigrated:         false,
	}
}

func RandomClinicCreate() *creator.CreateClinic {
	userId := Faker.UUID().V4()
	clinic := RandomClinic()
	clinic.Admins = nil
	clinic.CanonicalShareCode = nil
	clinic.ShareCodes = nil

	return &creator.CreateClinic{
		Clinic:            *clinic,
		CreatorUserId:     userId,
		CreateDemoPatient: false,
	}
}
