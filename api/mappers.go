package api

import (
	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
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
		Email:        &c.Email,
		Name:         &c.Name,
		PhoneNumbers: &phoneNumbers,
		PostalCode:   c.PostalCode,
		State:        c.State,
	}
}

func NewClinicUpdate(attributes Attributes) *clinics.Clinic {
	var phoneNumbers []clinics.PhoneNumber
	if attributes.PhoneNumbers != nil {
		for _, n := range *attributes.PhoneNumbers {
			phoneNumbers = append(phoneNumbers, clinics.PhoneNumber{
				Number: n.Number,
				Type:   n.Type,
			})
		}
	}

	update := &clinics.Clinic{
		Address:      attributes.Address,
		City:         attributes.City,
		ClinicType:   attributes.ClinicType,
		Country:      attributes.Country,
		Name:         attributes.Name,
		PhoneNumbers: &phoneNumbers,
		PostalCode:   attributes.PostalCode,
		State:        attributes.State,
	}
	if attributes.Email != nil {
		update.Email = strp(string(*attributes.Email))
	}

	return update
}

func NewClinicDto(c *clinics.Clinic) Clinic {
	dto := Clinic{}
	dto.Id = strp(c.Id.Hex())
	dto.Address = c.Address
	dto.City = c.City
	dto.ClinicType = c.ClinicType
	dto.Email = *c.Email
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
	dto := Clinician{}
	if clinician.UserId != nil {
		id := UserId(*clinician.UserId)
		dto.Id = &id
	}
	if clinician.Email != nil {
		dto.Email = openapi_types.Email(*clinician.Email)
	}
	dto.Name = clinician.Name
	dto.InviteId = clinician.InviteId
	dto.Permissions = ClinicianPermissions{clinician.Permissions...}
}

func strp(s string) *string {
	return &s
}
