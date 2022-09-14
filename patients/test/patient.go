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
