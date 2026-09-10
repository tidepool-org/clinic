package patients

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	clinicErrs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
)

var (
	ErrCSVHeaderEmpty             = errors.New("header is empty")
	ErrCSVHeaderMissingCols       = errors.New("header is missing columns")
	ErrCSVEmpty                   = errors.New("no rows in input records")
	ErrCSVNoPatientRows           = errors.New("no patient rows")
	ErrCSVPatientMissingName      = errors.New("missing name")
	ErrCSVPatientMissingBirthdate = errors.New("missing birthdate")
	ErrCSVPatientMissingMrn       = errors.New("missing MRN")
	ErrCSVPatientInvalidEmail     = errors.New("invalid email")
	ErrCSVPatientDuplicateMRN     = errors.New("duplicate MRN")
	ErrCSVPatientDuplicateEmail   = errors.New("duplicate email")
)

const (
	// Hardcoding column order for phase 1
	// Required fields/columns
	ColName = iota
	ColBirthdate
	ColMrn

	NumRequiredColumns
)

const (
	// Optional Columns
	ColEmail = iota + NumRequiredColumns
	ColDiagnosisType
	ColGlycemicPreset

	MaxInputColumns
)

const (
	// Output CSV is returned w/ original rows along with 2 additional columns
	OutputColStatus = MaxInputColumns + iota
	OutputColEmailed
	NumOutputCols
)

func ValidateCSVHeader(header []string) error {
	if len(header) == 0 {
		return ErrCSVHeaderEmpty
	}
	if len(header) < int(NumRequiredColumns) {
		return fmt.Errorf(`%w: only have %d columns, wanted %d`, ErrCSVHeaderMissingCols, len(header), NumRequiredColumns)
	}
	return nil
}

// ParsedCSVPatient represents the potential patient to be created from a CSV
// row along with a copy of the original input CSV row with added information
// in [Columns]. Patient may be nil if there is an error with the patient,
// which would be noted in [OutputColStatus] of [Columns]
type ParsedCSVPatient struct {
	Columns []string
	// Errs is an accumulated slice of errors that have been encountered while
	// parsing or creating a patient.
	Errs    []error
	Patient *Patient
}

// Err returns the accumulated errors encountered for a patient
func (p *ParsedCSVPatient) Err() error {
	return errors.Join(p.Errs...)
}

func (p *ParsedCSVPatient) AppendErr(err error) {
	if err == nil {
		return
	}
	if !errors.Is(p.Err(), err) {
		p.Errs = append(p.Errs, err)
	}
}

// ParsePotentialCSVPatients takes an input of slices of string slices (from a
// CSV or otherwise) representing data for a patient in a predefined order and
// returns the updated CSV to be used for output as well as the patients to
// be created along with their row information in parsedPatients. Individual
// errors that would prevent a patient from being created but that would NOT
// stop other patients from being created, if any, are outputed in the "reason"
// column defined as the slice index [OutputColStatus] in
// [ParsedCSVPatient.Columns] in which case [ParsedCSVPatient.Patient] would be
// empty. The CSV header is ALWAYS expected.
func ParsePotentialCSVPatients(ctx context.Context, patientSvc Service, userSvc UserService, csvRecords [][]string, clinicId primitive.ObjectID, invitedBy *string) (outputRows [][]string, outputHeader []string, parsedPatients []*ParsedCSVPatient, err error) {
	if len(csvRecords) == 0 {
		return nil, nil, nil, ErrCSVEmpty
	}
	header := csvRecords[0]
	if err := ValidateCSVHeader(header); err != nil {
		return nil, nil, nil, err
	}
	outputHeader = make([]string, NumOutputCols)
	copy(outputHeader, header)
	outputHeader[OutputColStatus] = "Reason"
	outputHeader[OutputColEmailed] = "Emailed?"
	outputRows = append(outputRows, outputHeader)

	filter := Filter{
		ClinicId:                                 strp(string(clinicId.Hex())),
		ExcludeSummaryExceptFieldsInMergeReports: true,
	}
	page := store.Pagination{Limit: 50_000, Offset: 0}
	res, err := patientSvc.List(ctx, &filter, page, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(`error fetching patients from clinic: %w`, err)
	}
	// Track duplicate MRNs/emails among the CSV rows itself (e.g., having the
	// same MRN in the CSV multiple times) and existing patients.
	mrnCounts := map[string]int{}
	emailCounts := map[string]int{}
	for _, p := range res.Patients {
		if mrn := pstr(p.Mrn); mrn != "" {
			mrnCounts[mrn]++
		}
		// Note emails of patients in clinic does not include all user who have a
		// specific email so a check for the email should be done via the user
		// service.
	}

	for _, record := range csvRecords[1:] {
		outputRow := make([]string, NumOutputCols)
		copy(outputRow[:MaxInputColumns], record)
		patient, err := NewPatientFromColumns(record, clinicId, invitedBy)
		parsedPatient := &ParsedCSVPatient{
			Patient: patient,
		}
		if err != nil {
			parsedPatient.AppendErr(err)
		} else {
			if !patient.GlycemicRanges.IsZero() {
				outputRow[ColGlycemicPreset] = string(patient.GlycemicRanges.Preset)
			}
			if !patient.DiagnosisType.IsZero() {
				outputRow[ColDiagnosisType] = string(*patient.DiagnosisType)
			}
			// No error from column data, now check for issues with mrn / email of
			// which there can be multiple.
			mrnCounts[*patient.Mrn]++
			if mrnCounts[*patient.Mrn] > 1 {
				parsedPatient.AppendErr(ErrCSVPatientDuplicateMRN)
			}
			if email := strings.ToLower(pstr(patient.Email)); email != "" {
				duplicateEmail := false
				emailCounts[email]++
				if emailCounts[email] > 1 {
					duplicateEmail = true
				} else {
					user, err := userSvc.GetUser(email)
					if err != nil && !errors.Is(err, clinicErrs.NotFound) {
						return nil, nil, nil, fmt.Errorf(`system error checking email: %w`, err)
					} else if user != nil {
						duplicateEmail = true
					}
				}
				if duplicateEmail {
					parsedPatient.AppendErr(ErrCSVPatientDuplicateEmail)
				}
			}
		}
		parsedPatient.Columns = outputRow
		parsedPatients = append(parsedPatients, parsedPatient)
		outputRows = append(outputRows, outputRow)
	}
	// Reiterate through patients to handle first instance of any individually
	// repeated MRN or email within the CSV itself (e.g., multiple cases of
	// email "dev@tidepool.org" within the CSV but not associated with an
	// existing patient.)
	for _, pp := range parsedPatients {
		if pp.Patient != nil && mrnCounts[*pp.Patient.Mrn] > 1 {
			pp.AppendErr(ErrCSVPatientDuplicateMRN)
		}

		if pp.Patient != nil {
			if email := strings.ToLower(pstr(pp.Patient.Email)); email != "" && emailCounts[email] > 1 {
				pp.AppendErr(ErrCSVPatientDuplicateEmail)
			}
		}
		if len(pp.Errs) > 0 {
			errs := make([]string, 0, len(pp.Errs))
			for _, err := range pp.Errs {
				errs = append(errs, err.Error())
			}
			pp.Columns[OutputColStatus] = strings.Join(errs, ", ")
			// set Patient to nil to indicate no further action to be done on patient
			pp.Patient = nil
		}
	}
	return outputRows, outputHeader, parsedPatients, nil
}

// CreateCSVPatients takes in a slice of ParsedCSVPatient and creates a patient
// for each one. It returns a slice of errors corresponding to the patient at
// each slice index as failure to create a single patient doesn't prevent other
// patients from being created. It also returns updated CSV output columns as
// some errors (db-related, system related, etc) can only surface during
// creation time.
func CreateCSVPatients(ctx context.Context, patientSvc Service, header []string, patients []*ParsedCSVPatient) (outputRows [][]string) {
	outputRows = make([][]string, 0, len(patients)+1)
	outputRows = append(outputRows, header)
	for _, parsedPatient := range patients {
		if parsedPatient.Patient == nil {
			if errors.Is(parsedPatient.Err(), ErrCSVPatientInvalidEmail) {
				parsedPatient.Columns[OutputColEmailed] = "N (invalid email)"
			} else {
				parsedPatient.Columns[OutputColEmailed] = "N"
			}
		} else {
			_, err := patientSvc.Create(ctx, *parsedPatient.Patient)
			if err != nil {
				status := parsedPatient.Columns[OutputColStatus]
				if status != "" {
					status += ", "
				}
				status += err.Error()
				parsedPatient.Columns[OutputColStatus] = status
				parsedPatient.Columns[OutputColEmailed] = "N"
			} else if parsedPatient.Patient.Email != nil && *parsedPatient.Patient.Email != "" {
				// Since the actual emaling is done outside the clinic service by
				// hydrophone, we assume any patients with emails that were
				// successfully created to have been emailed.
				parsedPatient.Columns[OutputColEmailed] = "Y"
			} else {
				parsedPatient.Columns[OutputColEmailed] = "N"
			}
		}
		outputRows = append(outputRows, parsedPatient.Columns)
	}
	return outputRows
}

// NewPatientFromColumns instantiates a Patient object suitable for actual
// creation given a row of text columns. Optionally, [Patient.InvitedBy] will
// be set to the user id invitedBy if it is non-nil the string value is
// non-zero.
func NewPatientFromColumns(record []string, clinicId primitive.ObjectID, invitedBy *string) (*Patient, error) {
	if len(record) < int(NumRequiredColumns) {
		return nil, fmt.Errorf(`Row has fewer than the minimum required columns: %v`, record)
	}
	fullName := strings.TrimSpace(record[ColName])
	if fullName == "" {
		return nil, ErrCSVPatientMissingName
	}
	birthdateRaw := strings.TrimSpace(record[ColBirthdate])
	if birthdateRaw == "" {
		return nil, ErrCSVPatientMissingBirthdate
	}
	birthDate, err := time.Parse(time.DateOnly, birthdateRaw)
	if err != nil {
		return nil, fmt.Errorf(`error parsing column "%s" as date: %w`, birthdateRaw, err)
	}
	mrn := strings.TrimSpace(record[ColMrn])
	if mrn == "" {
		return nil, ErrCSVPatientMissingMrn
	}
	var email string
	if len(record) > ColEmail {
		email = strings.TrimSpace(record[ColEmail])
		if email != "" {
			if _, err := mail.ParseAddress(email); err != nil {
				return nil, ErrCSVPatientInvalidEmail
			}
		}
	}
	var diagnosisType *DiagnosisType
	if len(record) > ColDiagnosisType {
		raw := strings.TrimSpace(record[ColDiagnosisType])
		if raw != "" {
			dt, err := ParseDiagnosisType(record[ColDiagnosisType])
			if err != nil {
				return nil, err
			}
			diagnosisType = &dt
		}
	}
	preset := DefaultGlycemicPreset
	if len(record) > ColGlycemicPreset {
		presetRaw := record[ColGlycemicPreset]
		preset = ParseGlycemicRangesPreset(presetRaw, DefaultGlycemicPreset)
	}

	patient := Patient{
		FullName:      &fullName,
		ClinicId:      &clinicId,
		BirthDate:     strp(birthDate.Format(time.DateOnly)),
		Mrn:           &mrn,
		Email:         strpnotzero(email),
		DiagnosisType: diagnosisType,
		InvitedBy:     invitedBy,
		GlycemicRanges: GlycemicRanges{
			Type:   GlycemicRangeTypePreset,
			Preset: GlycemicRangesPreset(preset),
		},
		Permissions: &CustodialAccountPermissions,
	}
	return &patient, nil
}

func pstr(p *string) string {
	if p == nil {
		return ""
	}

	return *p
}

func strp(s string) *string {
	return &s
}

func strpnotzero(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
