package xealth

import (
	"errors"
	errs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"strings"
)

type DuplicateEmailValidator struct {
	users patients.UserService
}

func (d *DuplicateEmailValidator) IsDuplicate(email string) (bool, error) {
	user, err := d.users.GetUser(email)
	if errors.Is(err, errs.NotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return user != nil && user.UserID != "", nil
}

type GuardianDataValidator struct {
	duplicateEmailValidator *DuplicateEmailValidator
}

func NewGuardianDataValidator(users patients.UserService) *GuardianDataValidator {
	return &GuardianDataValidator{
		duplicateEmailValidator: &DuplicateEmailValidator{users: users},
	}
}

func (g *GuardianDataValidator) Validate(d GuardianFormData) (GuardianFormValidationErrors, error) {
	formValidationErrors := GuardianFormValidationErrors{}
	if strings.TrimSpace(d.Guardian.FirstName) == "" {
		formValidationErrors.FormHasErrors = true
		formValidationErrors.Guardian.FirstName = NewValidationError("First name is required")
	}
	if strings.TrimSpace(d.Guardian.LastName) == "" {
		formValidationErrors.FormHasErrors = true
		formValidationErrors.Guardian.LastName = NewValidationError("Last name is required")
	}
	if !isValidEmail(d.Guardian.Email) {
		formValidationErrors.FormHasErrors = true
		formValidationErrors.Guardian.Email = NewValidationError("The email address is not valid")
	} else if duplicate, err := g.duplicateEmailValidator.IsDuplicate(d.Guardian.Email); err != nil {
		return formValidationErrors, err
	} else if duplicate {
		formValidationErrors.FormHasErrors = true
		formValidationErrors.Guardian.Email = NewValidationError("A user with this email address already exists")
	}

	return formValidationErrors, nil
}

type PatientDataValidator struct {
	duplicateEmailValidator *DuplicateEmailValidator
}

func NewPatientDataValidator(users patients.UserService) *PatientDataValidator {
	return &PatientDataValidator{
		duplicateEmailValidator: &DuplicateEmailValidator{users: users},
	}
}

func (g *PatientDataValidator) Validate(d PatientFormData) (PatientFormValidationErrors, error) {
	formValidationErrors := PatientFormValidationErrors{}
	if !isValidEmail(d.Patient.Email) {
		formValidationErrors.FormHasErrors = true
		formValidationErrors.Patient.Email = NewValidationError("The email address is not valid")
	} else if duplicate, err := g.duplicateEmailValidator.IsDuplicate(d.Patient.Email); err != nil {
		return formValidationErrors, err
	} else if duplicate {
		formValidationErrors.FormHasErrors = true
		formValidationErrors.Patient.Email = NewValidationError("A user with this email address already exists")
	}

	return formValidationErrors, nil
}
