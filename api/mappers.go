package api

import (
	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"time"
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

	return &clinics.Clinic{
		Address:      c.Address,
		City:         c.City,
		ClinicType:   c.ClinicType,
		Country:      c.Country,
		Email:        strp(string(c.Email)),
		Name:         &c.Name,
		PhoneNumbers: &phoneNumbers,
		PostalCode:   c.PostalCode,
		State:        c.State,
	}
}

func NewClinicDto(c *clinics.Clinic) Clinic {
	dto := Clinic{}
	dto.Id = Id(c.Id.Hex())
	dto.Address = c.Address
	dto.City = c.City
	dto.ClinicType = c.ClinicType
	dto.Email = openapi_types.Email(*c.Email)
	dto.Name = *c.Name
	dto.PostalCode = c.PostalCode
	dto.State = c.State

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
		UserId:   clinician.UserId,
		Name:     clinician.Name,
		InviteId: clinician.InviteId,
	}
	if clinician.Email != nil {
		dto.Email = openapi_types.Email(*clinician.Email)
	}
	if clinician.Permissions != nil {
		perms := ClinicianPermissions(*clinician.Permissions)
		dto.Permissions = &perms
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
	email := string(clinician.Email)
	c := &clinicians.Clinician{
		Name:     clinician.Name,
		UserId:   clinician.UserId,
		InviteId: clinician.InviteId,
		Email:    &email,
	}
	if clinician.Permissions != nil {
		perms := []string(*clinician.Permissions)
		c.Permissions = &perms
	}
	return c
}

func NewClinicianUpdate(clinician Clinician) *clinicians.Clinician {
	c := &clinicians.Clinician{
		Name: clinician.Name,
	}
	if clinician.Permissions != nil {
		perms := []string(*clinician.Permissions)
		c.Permissions = &perms
	}
	return c
}

func NewPatientDto(patient *patients.Patient) Patient {
	return Patient{
		BirthDate:     strtodatep(patient.BirthDate),
		Email:         strtoemailp(patient.Email),
		FullName:      patient.FullName,
		Id:            *strtouserid(patient.UserId),
		Mrn:           patient.Mrn,
		Permissions:   permissions(patient.Permissions),
		TargetDevices: patient.TargetDevices,
	}
}

func NewPatientUpdate(patient Patient) patients.Patient {
	p := patients.Patient{
		FullName:      patient.FullName,
		Mrn:           patient.Mrn,
		TargetDevices: patient.TargetDevices,
	}
	if patient.Email != nil {
		p.Email = strp(string(*patient.Email))
	}
	if patient.BirthDate != nil {
		p.BirthDate = strp(patient.BirthDate.Format(dateFormat))
	}
	return p
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

const dateFormat = "2006-01-02"

func strtodatep(s *string) *openapi_types.Date {
	if s == nil {
		return nil
	}
	t, err := time.Parse(dateFormat, *s)
	if err != nil {
		return nil
	}
	return &openapi_types.Date{Time: t}
}

func strtoemailp(s *string) *openapi_types.Email {
	if s == nil {
		return nil
	}
	email := openapi_types.Email(*s)
	return &email
}

func strtouserid(s *string) *UserId {
	if s == nil {
		return nil
	}
	id := UserId(*s)
	return &id
}

func permissions(p *patients.PatientPermissions) *PatientPermissions {
	if p == nil {
		return nil
	}
	return &PatientPermissions{
		Custodian: p.Custodian,
		Note:      p.Note,
		Root:      p.Root,
		Upload:    p.Upload,
		View:      p.View,
	}
}

func strp(s string) *string {
	return &s
}
