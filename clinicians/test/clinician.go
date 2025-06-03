package test

import (
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/test"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func RandomClinician() *clinicians.Clinician {
	clinicId := primitive.NewObjectID()
	userId := test.Faker.UUID().V4()
	email := test.Faker.Internet().Email()
	name := test.Faker.Person().Name()
	roles := []string{test.Faker.RandomStringElement([]string{"CLINIC_MEMBER", "CLINIC_ADMIN"})}

	return &clinicians.Clinician{
		ClinicId: &clinicId,
		UserId:   &userId,
		Email:    &email,
		Name:     &name,
		Roles:    roles,
	}
}

func RandomClinicianInvite() *clinicians.Clinician {
	inviteId := test.Faker.UUID().V4()
	clinician := RandomClinician()
	clinician.UserId = nil
	clinician.Name = nil
	clinician.InviteId = &inviteId

	return clinician
}
