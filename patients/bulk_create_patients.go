package patients

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	clinicErrs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
)

var (
	ErrCSVUnreadable       = &BlockingInputCSVError{"unable to parse or read CSV"}
	ErrCSVFewerThanMinCols = &BlockingInputCSVError{"row has less than the minimum number of required columns"}
	ErrCSVEmpty            = &BlockingInputCSVError{"empty CSV"}

	ErrCSVPatientMissingName           = &BlockingInputCSVError{"missing name"}
	ErrCSVPatientMissingBirthdate      = &BlockingInputCSVError{"missing birthdate"}
	ErrCSVPatientMissingMRN            = &BlockingInputCSVError{"missing MRN"}
	ErrCSVPatientInvalidBirthdate      = &ExcludedPatientInputError{"invalid birthdate"}
	ErrCSVPatientInvalidEmail          = errors.New("invalid email")
	ErrCSVPatientInvalidDiagnosisType  = errors.New("invalid diagnosis type")
	ErrCSVPatientInvalidGlycemicPreset = errors.New("invalid glycemic preset")
	ErrCSVPatientDuplicateMRN          = &ExcludedPatientInputError{"duplicate MRN"}
	ErrCSVPatientDuplicateEmail        = &ExcludedPatientInputError{"duplicate email"}
)

const (
	// Hardcoding column order for phase 1
	// Required fields/columns
	ColName = iota
	ColBirthdate
	ColMRN

	NumRequiredColumns
)

// ExcludedPatientInputError indicates an error that should preclude creating a
// individual patient/row. It does not affect the creation of others.
type ExcludedPatientInputError struct {
	Message string
}

func (e *ExcludedPatientInputError) Error() string {
	return e.Message
}

// BlockingInputCSVError indicates an error that should preclude creating any
// and all patients due to an issue with the CSV.
type BlockingInputCSVError struct {
	Message string
}

func (e *BlockingInputCSVError) Error() string {
	return e.Message
}

const (
	// Optional Columns
	ColEmail = iota + NumRequiredColumns
	ColDiagnosisType
	ColGlycemicPreset

	MaxInputColumns
)

const (
	// Output CSV is returned w/ original rows along with 2 additional columns
	OutputColErrors = MaxInputColumns + iota
	OutputColEmailed
	OutputColState
	NumOutputCols
)

// IsBulkPatientBlockingInputError returns true if an error is a fatal CSV
// validation error. This occurs only when required columns are missing.
// Invalid values are handled separately and are "logged" errors within the CSV
// output error column, but do not block further processing of other rows..
func IsBulkPatientBlockingInputError(err error) bool {
	var blockingErr *BlockingInputCSVError
	return errors.As(err, &blockingErr)
}

// ShouldSkipPatientCreation returns true if the given error is an error
// that does not allow creation of a patient.
func ShouldSkipPatientCreation(err error) bool {
	if IsBulkPatientBlockingInputError(err) {
		return true
	}
	var excludedErr *ExcludedPatientInputError
	return errors.As(err, &excludedErr)
}

// ParsedCSVPatient represents the potential patient to be created from a CSV
// row along with a copy of the original input CSV row with added information
// in [Columns]. Patient may be nil if there is an error with the patient,
// which would be noted in [OutputColErrors] of [Columns]
type ParsedCSVPatient struct {
	Columns []string
	// Errs is an accumulated slice of errors that have been encountered
	// while parsing or creating a patient.
	Errs    []error
	Patient *Patient
}

// Err returns the accumulated errors encountered for a patient
func (p *ParsedCSVPatient) Err() error {
	return errors.Join(p.Errs...)
}

func (p *ParsedCSVPatient) ErrorsDescription() string {
	errs := make([]string, 0, len(p.Errs))
	for _, err := range p.Errs {
		errs = append(errs, err.Error())
	}
	return strings.Join(errs, ", ")
}

func (p *ParsedCSVPatient) AppendErr(err error) {
	if err == nil {
		return
	}
	if !errors.Is(p.Err(), err) {
		p.Errs = append(p.Errs, err)
	}
}

func (p *ParsedCSVPatient) Emailable() bool {
	err := p.Err()
	return p.Patient != nil && pstr(p.Patient.Email) != "" && !errors.Is(err, ErrCSVPatientDuplicateEmail) && !errors.Is(err, ErrCSVPatientInvalidEmail)
}

// ParsePotentialCSVPatients takes an input of slices of string slices (from a
// HEADERLESS CSV or otherwise) representing data for a patient in the same
// patient field column-order as [ColName], [ColBirthdate], [ColMRN] and
// returns the updated CSV to be used for output as well as the patients to be
// created along with their row information in parsedPatients. Individual
// errors that would prevent a patient from being created but that would NOT
// stop other patients from being created, if any, are outputed in the "reason"
// column defined as the slice index [OutputColErrors] in
// [ParsedCSVPatient.Columns] in which case [ParsedCSVPatient.Patient] would be
// empty. There should be NO CSV input header row. A returned error can be
// checked with [IsBulkPatientBlockingInputError] to indicate to the caller an input
// error (bad request/malformed CSV) that should avoid attempting to create
// any patients.
func ParsePotentialCSVPatients(ctx context.Context, patientSvc Service, userSvc UserService, csvRecords [][]string, clinicId primitive.ObjectID, invitedBy *string) (outputRows [][]string, outputHeader []string, parsedPatients []*ParsedCSVPatient, err error) {
	if len(csvRecords) == 0 {
		return nil, nil, nil, ErrCSVEmpty
	}
	outputHeader = []string{
		"Name",
		"Birthdate",
		"MRN",
		"Email",
		"Diabetes Type",
		"Glycemic Target",
		"Reason",
		"Emailed?",
		"Status",
	}
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
		// specific email so the actual check for the email should be done via the user
		// service a preliminary check of the same email within a clinic is OK.
		if email := strings.ToLower(pstr(p.Email)); email != "" {
			emailCounts[email]++
		}
	}

	// Keep track of a fatal error but don't immediately return on one because
	// still want to process the other rows for potential problems so that each
	// individual row's errors can be checked as well.
	var fatalErrs []error
	for i, record := range csvRecords {
		rowNum := i + 1
		outputRow := make([]string, NumOutputCols)
		copy(outputRow[:MaxInputColumns], record)
		patient, err := NewPatientFromColumns(rowNum, record, clinicId, invitedBy)
		parsedPatient := &ParsedCSVPatient{
			Patient: patient,
		}
		if err != nil {
			parsedPatient.AppendErr(err)
			if IsBulkPatientBlockingInputError(err) {
				fatalErrs = append(fatalErrs, err)
			}
		}
		if patient != nil && !IsBulkPatientBlockingInputError(err) {
			// Checking for duplicate MRN's and other validation issues will only
			// happen if there was no fatal error for the patient's info in the CSV
			if !patient.GlycemicRanges.IsZero() {
				outputRow[ColGlycemicPreset] = string(patient.GlycemicRanges.Preset)
			} else {
				outputRow[ColGlycemicPreset] = ""
			}
			if !patient.DiagnosisType.IsZero() {
				outputRow[ColDiagnosisType] = string(*patient.DiagnosisType)
			} else {
				outputRow[ColDiagnosisType] = ""
			}
			// No error from column data, now check for issues with mrn / email of
			// which there can be multiple.
			if mrn := pstr(patient.Mrn); mrn != "" {
				mrnCounts[mrn]++
				if mrnCounts[mrn] > 1 {
					parsedPatient.AppendErr(ErrCSVPatientDuplicateMRN)
				}
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
		if mrn := pstr(pp.Patient.Mrn); mrn != "" && mrnCounts[*pp.Patient.Mrn] > 1 {
			pp.AppendErr(ErrCSVPatientDuplicateMRN)
		}
		if email := strings.ToLower(pstr(pp.Patient.Email)); email != "" && emailCounts[email] > 1 {
			pp.AppendErr(ErrCSVPatientDuplicateEmail)
		}
		pp.Columns[OutputColErrors] = pp.ErrorsDescription()
	}
	return outputRows, outputHeader, parsedPatients, errors.Join(fatalErrs...)
}

// ParsePotentialCSVPatientsReader wraps [ParsePotentialCSVPatients] and takes
// the incoming [io.Reader] r and parses it into a slice of string slices.
func ParsePotentialCSVPatientsReader(ctx context.Context, r io.Reader, patientSvc Service, userSvc UserService, clinicId primitive.ObjectID, invitedBy *string) (outputRows [][]string, outputHeader []string, parsedPatients []*ParsedCSVPatient, err error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, nil, fmt.Errorf(`%w: %w`, ErrCSVUnreadable, err)
	}
	return ParsePotentialCSVPatients(ctx, patientSvc, userSvc, records, clinicId, invitedBy)
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
	numSkipped := 0
	numFailedCreation := 0
	numSkippedDupeEmail := 0
	numSkippedDupeMRN := 0
	numCreated := 0
	numEmailed := 0
	for _, parsedPatient := range patients {
		patientErrs := parsedPatient.Err()
		emailed := false
		if errors.Is(patientErrs, ErrCSVPatientDuplicateEmail) {
			numSkippedDupeEmail++
		}
		if errors.Is(patientErrs, ErrCSVPatientDuplicateMRN) {
			numSkippedDupeMRN++
		}
		if ShouldSkipPatientCreation(patientErrs) {
			numSkipped++
			parsedPatient.Columns[OutputColState] = "skipped"
		} else {
			_, err := patientSvc.Create(ctx, *parsedPatient.Patient)
			if err != nil {
				parsedPatient.AppendErr(err)
				parsedPatient.Columns[OutputColErrors] = parsedPatient.ErrorsDescription()
				numFailedCreation++
				parsedPatient.Columns[OutputColEmailed] = "N"
				parsedPatient.Columns[OutputColState] = "failed"
			} else {
				numCreated++
				parsedPatient.Columns[OutputColState] = "created"
				if parsedPatient.Emailable() {
					// Since the actual emaling is done outside the clinic service by
					// hydrophone, we assume any patients with emails that were
					// successfully created to have been emailed.
					numEmailed++
					emailed = true
				}
			}
		}
		if emailed {
			parsedPatient.Columns[OutputColEmailed] = "Y"
		} else if errors.Is(patientErrs, ErrCSVPatientInvalidEmail) {
			parsedPatient.Columns[OutputColEmailed] = "N (invalid email)"
		} else {
			parsedPatient.Columns[OutputColEmailed] = "N"
		}
		outputRows = append(outputRows, parsedPatient.Columns)
	}
	outputRows = append(outputRows, [][]string{
		{"Patients Processed", fmt.Sprintf("%v", len(patients))},
		{"Patients Created", fmt.Sprintf("%v", numCreated)},
		{"Patients Skipped", fmt.Sprintf("%v", numSkipped)},
		{"Patients Emailed", fmt.Sprintf("%v", numEmailed)},
		{"Patients Failed Creation", fmt.Sprintf("%v", numFailedCreation)},
		{"Duplicate MRNs count", fmt.Sprintf("%v", numSkippedDupeMRN)},
		{"Duplicate emails count", fmt.Sprintf("%v", numSkippedDupeEmail)},
	}...)
	return outputRows
}

// NewPatientFromColumns instantiates a Patient object suitable for actual
// creation given a row of text columns. Optionally, [Patient.InvitedBy] will
// be set to the user id invitedBy if it is non-nil the string value is
// non-zero. Any error returned should be checked with
// [IsBulkPatientBlockingInputError] as the input record may have validation errors
// that still allow a patient to be created and the error will then be recorded
// in the output CSV.
func NewPatientFromColumns(rowNum int, record []string, clinicId primitive.ObjectID, invitedBy *string) (*Patient, error) {
	var errs []error
	if len(record) < int(NumRequiredColumns) {
		errs = append(errs, fmt.Errorf(`%w: row %v`, ErrCSVFewerThanMinCols, rowNum))
	}
	var fullName string
	if len(record) > ColName {
		fullName = strings.TrimSpace(record[ColName])
	}
	if fullName == "" {
		errs = append(errs, fmt.Errorf(`%w: row %v, column %v`, ErrCSVPatientMissingName, rowNum, ColName+1))
	}

	var birthdateRaw string
	var birthDate time.Time
	if len(record) > ColBirthdate {
		birthdateRaw = strings.TrimSpace(record[ColBirthdate])
	}
	if birthdateRaw == "" {
		errs = append(errs, fmt.Errorf(`%w: row %v, column %v`, ErrCSVPatientMissingBirthdate, rowNum, ColBirthdate+1))
	} else {
		var err error
		birthDate, err = time.Parse(time.DateOnly, birthdateRaw)
		if err != nil {
			errs = append(errs, fmt.Errorf(`%w: row %v, column %v`, ErrCSVPatientInvalidBirthdate, rowNum, ColBirthdate+1))
		}
	}
	var mrn string
	if len(record) > ColMRN {
		mrn = strings.TrimSpace(record[ColMRN])
	}
	if mrn == "" {
		errs = append(errs, fmt.Errorf(`%w: row %v, column %v`, ErrCSVPatientMissingMRN, rowNum, ColMRN+1))
	}
	var email string
	if len(record) > ColEmail {
		emailRaw := strings.TrimSpace(record[ColEmail])
		if emailRaw != "" {
			if _, err := mail.ParseAddress(emailRaw); err != nil {
				errs = append(errs, fmt.Errorf(`%w: row %v, column %v`, ErrCSVPatientInvalidEmail, rowNum, ColEmail+1))
			} else {
				email = emailRaw
			}
		}
	}
	var diagnosisType *DiagnosisType
	if len(record) > ColDiagnosisType {
		raw := strings.TrimSpace(record[ColDiagnosisType])
		if raw != "" {
			dt, err := ParseDiagnosisType(record[ColDiagnosisType])
			// Diagnosis type is only set if it's one of the supported types in [ValidDiagnosisTypes] AND it's not the type non applicable, [DiagnosisTypeNotApplicable], otherwise it is set to blank / nil.
			if err == nil && dt != DiagnosisTypeNotApplicable {
				diagnosisType = &dt
			} else if err != nil {
				errs = append(errs, ErrCSVPatientInvalidDiagnosisType)
			}
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
	return &patient, errors.Join(errs...)
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
