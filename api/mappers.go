package api

import (
	"fmt"
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
		Roles:    ClinicianRoles(clinician.Roles),
	}
	if clinician.Email != nil {
		dto.Email = openapi_types.Email(*clinician.Email)
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
	return &clinicians.Clinician{
		Name:     clinician.Name,
		UserId:   clinician.UserId,
		InviteId: clinician.InviteId,
		Roles:    clinician.Roles,
		Email:    strp(string(clinician.Email)),
	}
}

func NewClinicianUpdate(clinician Clinician) *clinicians.Clinician {
	return &clinicians.Clinician{
		Name:  clinician.Name,
		Roles: clinician.Roles,
	}
}

func NewPatientDto(patient *patients.Patient) Patient {
	return Patient{
		BirthDate:     strtodatep(patient.BirthDate),
		Email:         strtoemailp(patient.Email),
		FullName:      patient.FullName,
		Id:            *strtouserid(patient.UserId),
		Mrn:           patient.Mrn,
		Permissions:   NewPermissionsDto(patient.Permissions),
		TargetDevices: patient.TargetDevices,
	}
}

func NewPermissions(dto *PatientPermissions) *patients.Permissions {
	var permissions *patients.Permissions
	if dto != nil {
		permissions = &patients.Permissions{}
		if dto.Custodian != nil {
			permissions.Custodian = &patients.Permission{}
		}
		if dto.Upload != nil {
			permissions.Upload = &patients.Permission{}
		}
		if dto.Note != nil {
			permissions.Note = &patients.Permission{}
		}
		if dto.View != nil {
			permissions.View = &patients.Permission{}
		}
	}
	return permissions
}

func NewPermissionsDto(dto *patients.Permissions) *PatientPermissions {
	var permissions *PatientPermissions
	if dto != nil {
		permissions = &PatientPermissions{}
		permission := make(map[string]interface{})
		if dto.Custodian != nil {
			permissions.Custodian = &permission
		}
		if dto.Upload != nil {
			permissions.Upload = &permission
		}
		if dto.Note != nil {
			permissions.Note = &permission
		}
		if dto.View != nil {
			permissions.View = &permission
		}
	}
	return permissions
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

func NewPatientClinicRelationshipsDto(patients []*patients.Patient, clinicList []*clinics.Clinic) (*[]PatientClinicRelationship, error) {
	clinicsMap := make(map[string]*clinics.Clinic, 0)
	for _, clinic := range clinicList {
		clinicsMap[clinic.Id.Hex()] = clinic
	}
	dtos := make([]PatientClinicRelationship, 0)
	for _, patient := range patients {
		clinic, ok := clinicsMap[patient.ClinicId.Hex()]
		if !ok || clinic == nil {
			return nil, fmt.Errorf("clinic not found")
		}

		dtos = append(dtos, PatientClinicRelationship{
			Clinic:  NewClinicDto(clinic),
			Patient: NewPatientDto(patient),
		})
	}
	return &dtos, nil
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

func strp(s string) *string {
	return &s
}
