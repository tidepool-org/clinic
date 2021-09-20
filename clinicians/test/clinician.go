package test

import (
	"github.com/jaswdr/faker"
	"github.com/onsi/ginkgo/config"
	"github.com/tidepool-org/clinic/clinicians"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math/rand"
)

var (
	Faker = faker.NewWithSeed(rand.NewSource(config.GinkgoConfig.RandomSeed))
)

func RandomClinician() *clinicians.Clinician {
	clinicId := primitive.NewObjectID()
	userId := Faker.UUID().V4()
	email := Faker.Internet().Email()
	name := Faker.Person().Name()
	roles := []string{Faker.RandomStringElement([]string{"CLINIC_MEMBER", "CLINIC_ADMIN"})}

	return &clinicians.Clinician{
		ClinicId: &clinicId,
		UserId:   &userId,
		Email:    &email,
		Name:     &name,
		Roles:    roles,
	}
}

func RandomClinicianInvite() *clinicians.Clinician {
	inviteId := Faker.UUID().V4()
	clinician := RandomClinician()
	clinician.UserId = nil
	clinician.Name = nil
	clinician.InviteId = &inviteId

	return clinician
}
