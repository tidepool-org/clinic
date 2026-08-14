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
	ErrCSVNoRows                  = errors.New("no rows in input records")
	ErrCSVNoPatientRows           = errors.New("no patient rows")
	ErrCSVNotEnoughColumns        = errors.New("not enough columns in input CSV")
	ErrCSVPatientMissingName      = errors.New("missing name")
	ErrCSVPatientMissingBirthdate = errors.New("missing birthdate")
	ErrCSVPatientMissingMrn       = errors.New("missing mrn")
	ErrCSVPatientInvalidEmail     = errors.New("invalid email")
	ErrCSVPatientDuplicateMrn     = errors.New("duplicate mrn")
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

func ValidateCSVHeader(records [][]string) error {
	if len(records) == 0 {
		return ErrCSVNoRows
	}
	if len(records) == 1 {
		return ErrCSVNoPatientRows
	}
	header := records[0]
	if len(header) < int(NumRequiredColumns) {
		return fmt.Errorf(`%w: only have %d columns`, ErrCSVNotEnoughColumns, len(header))
	}
	return nil
}

// ParsedCSVPatient represents the potential patient to be created from a CSV
// row along with a copy of the original input CSV row with added information
// in [Columns]. Patient may be nil if there is an error with the patient,
// which would be noted in [OutputColStatus] of [Columns]
type ParsedCSVPatient struct {
	Columns []string
	// Err is the associated errors of a patient used to check if a specific
	// issued occurred.
	Err     error
	Patient *Patient
}

// ParsePotentialCSVPatients takes an input of slices of string slices (from a
// CSV or otherwise) representing data for a patient in a predefined order and
// returns the updated CSV to be used for output as well as the patients to
// be created along with their row information in parsedPatients. Individual
// errors that would prevent a patient from being created but that would NOT
// stop other patients from being created, if any, are outputed in the "reason"
// column defined as the slice index [OutputColStatus] in
// [ParsedCSVPatient.Columns] in which case [ParsedCSVPatient.Patient] would be
// empty.
func ParsePotentialCSVPatients(ctx context.Context, patientSvc Service, userSvc UserService, CSVRecords [][]string, clinicId primitive.ObjectID) (outputRows [][]string, outputHeader []string, parsedPatients []ParsedCSVPatient, err error) {
	if err := ValidateCSVHeader(CSVRecords); err != nil {
		return nil, nil, nil, err
	}
	header := CSVRecords[0]
	outputHeader = make([]string, NumOutputCols)
	copy(outputHeader, header)
	outputHeader[OutputColStatus] = "Reason"
	outputHeader[OutputColEmailed] = "Emailed?"
	outputRows = append(outputRows, outputHeader)

	for _, record := range CSVRecords[1:] {
		outputRow := make([]string, NumOutputCols)
		copy(outputRow, record)
		var patientErr error
		patient, err := NewPatientFromColumns(record, clinicId)
		if err != nil {
			outputRow[OutputColStatus] = err.Error()
			patientErr = errors.Join(patientErr, err)
		} else {
			if !patient.GlycemicRanges.IsZero() {
				outputRow[ColGlycemicPreset] = string(patient.GlycemicRanges.Preset)
			}
			if !patient.DiagnosisType.IsZero() {
				outputRow[ColDiagnosisType] = string(*patient.DiagnosisType)
			}
			// No error from column data, now check for issues with mrn / email of
			// which there can be multiple.
			var issues []string
			page := store.Pagination{Limit: 1, Offset: 0}
			filter := Filter{
				ClinicId: strp(string(clinicId.Hex())),
				Mrn:      patient.Mrn,
			}
			res, err := patientSvc.List(ctx, &filter, page, nil)
			if err != nil {
				issues = append(issues, fmt.Sprintf(`system error checking mrn: %v`, err))
			} else if res != nil && res.MatchingCount > 0 {
				issues = append(issues, `duplicate MRN`)
				patientErr = errors.Join(patientErr, ErrCSVPatientDuplicateMrn)
			}
			if pstr(patient.Email) != "" {
				user, err := userSvc.GetUser(*patient.Email)
				if err != nil && !errors.Is(err, clinicErrs.NotFound) {
					issues = append(issues, fmt.Sprintf(`system error checking email: %v`, err))
				} else if user != nil {
					issues = append(issues, `duplicate email`)
				}
			}
			if len(issues) > 0 {
				outputRow[OutputColStatus] = strings.Join(issues, ", ")
				patient = nil
			}
		}
		parsedPatients = append(parsedPatients, ParsedCSVPatient{
			Columns: outputRow,
			Patient: patient,
			Err:     patientErr,
		})
		outputRows = append(outputRows, outputRow)
	}
	return outputRows, outputHeader, parsedPatients, nil
}

// CreateCSVPatients takes in a slice of ParsedCSVPatient and creates a patient
// for each one. It returns a slice of errors corresponding to the patient at
// each slice index as failure to create a single patient doesn't prevent other
// patients from being created. It also returns updated CSV output columns as
// some errors (db-related, system related, etc) can only surface during
// creation time.
func CreateCSVPatients(ctx context.Context, patientSvc Service, header []string, patients []ParsedCSVPatient) (outputRows [][]string) {
	outputRows = make([][]string, 0, len(patients)+1)
	outputRows = append(outputRows, header)
	for _, parsedPatient := range patients {
		if parsedPatient.Patient == nil {
			if errors.Is(parsedPatient.Err, ErrCSVPatientInvalidEmail) {
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
			} else {
				parsedPatient.Columns[OutputColEmailed] = "Y"
			}
		}
		outputRows = append(outputRows, parsedPatient.Columns)
	}
	return outputRows
}

// NewPatientFromColumns instantiates a Patient object suitable for actual
// creation given a row of text columns.
func NewPatientFromColumns(record []string, clinicId primitive.ObjectID) (*Patient, error) {
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
	var preset GlycemicRangesPreset
	if len(record) > ColGlycemicPreset {
		presetRaw := strings.TrimSpace(record[ColGlycemicPreset])
		preset = ParseGlycemicRangesPreset(presetRaw, DefaultGlycemicPreset)
	}

	patient := Patient{
		FullName:      &fullName,
		ClinicId:      &clinicId,
		BirthDate:     strp(birthDate.Format(time.DateOnly)),
		Mrn:           &mrn,
		Email:         strpnotzero(email),
		DiagnosisType: diagnosisType,
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
