package xealth

import (
	"errors"
	errs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"strings"
)

var (
	ParagraphSeparator             = "\n"
	DuplicateEmailAddressErrorText = "The email address you chose is already in use with another account in Tidepool. You could click “Cancel” and try to create the patient with a different email address.\nIf you think the patient already exists in Tidepool and would like to enroll that account for monitoring in Xealth, you could go to Tidepool and look for the account with the email you are trying to create. If it is the same patient, make sure the MRN and date of birth associated with the account in Tidepool match those values in this patient’s record in Xealth. If you change them in Tidepool to match and return to the patient’s record in Xealth, you can enter another order and it will enroll their existing Tidepool account for monitoring.\nIf you continue experiencing difficulties, please contact support@tidepool.org."
	SomethingWentWrongErrorText    = "Something went wrong when we tried to create a new patient account in Tidepool. Please click “Cancel” and try again. If you continue experiencing difficulties, please contact support@tidepool.org."
	ErrorTitle                     = "There Was a Problem Adding Patient To Tidepool"
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

func (g *GuardianDataValidator) Validate(d GuardianFormData) (FormErrors, error) {
	formErrors := &PreorderFormErrors{
		Title: ErrorTitle,
	}
	if strings.TrimSpace(d.Guardian.FirstName) == "" || strings.TrimSpace(d.Guardian.LastName) == "" || !isValidEmail(d.Guardian.Email) {
		formErrors.AddErrorParagraph(SomethingWentWrongErrorText)
	} else if duplicate, err := g.duplicateEmailValidator.IsDuplicate(d.Guardian.Email); err != nil {
		return nil, err
	} else if duplicate {
		paragraphs := strings.Split(DuplicateEmailAddressErrorText, ParagraphSeparator)
		for _, p := range paragraphs {
			formErrors.AddErrorParagraph(p)
		}
	}

	return formErrors, nil
}

type PatientDataValidator struct {
	duplicateEmailValidator *DuplicateEmailValidator
}

func NewPatientDataValidator(users patients.UserService) *PatientDataValidator {
	return &PatientDataValidator{
		duplicateEmailValidator: &DuplicateEmailValidator{users: users},
	}
}

func (g *PatientDataValidator) Validate(d PatientFormData) (FormErrors, error) {
	formErrors := &PreorderFormErrors{
		Title: ErrorTitle,
	}
	if !isValidEmail(d.Patient.Email) {
		formErrors.AddErrorParagraph(SomethingWentWrongErrorText)
	} else if duplicate, err := g.duplicateEmailValidator.IsDuplicate(d.Patient.Email); err != nil {
		return nil, err
	} else if duplicate {
		paragraphs := strings.Split(DuplicateEmailAddressErrorText, ParagraphSeparator)
		for _, p := range paragraphs {
			formErrors.AddErrorParagraph(p)
		}
	}

	return formErrors, nil
}
