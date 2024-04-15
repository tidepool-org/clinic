package merge

import (
	"context"
	"errors"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
)

const (
	PatientConflictCategoryDuplicateClaimed          = "Duplicate Claimed Accounts"
	PatientConflictCategoryLikelyDuplicateAccounts   = "Likely Duplicate Accounts"
	PatientConflictCategoryPossibleDuplicateAccounts = "Possible Duplicate Accounts"
	PatientConflictCategoryDuplicateMRNs             = "Duplicate MRNs Likely Typos"

	// PatientActionKeep is used for target patients when there's no corresponding patient in the source clinic
	PatientActionKeep = "KEEP"
	// PatientActionMerge is used when the source patient will be merged to a target patient record
	PatientActionMerge = "MERGE"
	// PatientActionMergeInto is when the target record will be the recipient of a merge
	PatientActionMergeInto = "MERGE_INTO"
	// PatientActionMove is used when the source patient will be moved to the target clinic
	PatientActionMove = "MOVE"
)

type PatientReportDetails struct {
	ConflictCategories []string
	PatientAction      string

	SourcePatient    patients.Patient
	TargetPatient    patients.Patient
	ResultingPatient patients.Patient
}

type TargetPatientMergeTask struct {
	patient patients.Patient

	source clinics.Clinic
	target clinics.Clinic

	service patients.Service

	result TaskResult[PatientReportDetails]
	err    error
}

func NewTargetPatientMergeTask(patient patients.Patient, source, target clinics.Clinic, service patients.Service) Task[PatientReportDetails] {
	return &TargetPatientMergeTask{
		patient: patient,
		source:  source,
		target:  target,
		service: service,
	}
}

func (s *TargetPatientMergeTask) CanRun() bool {
	return true
}

func (s *TargetPatientMergeTask) DryRun(ctx context.Context) error {
	s.result = TaskResult[PatientReportDetails]{
		ReportDetails: PatientReportDetails{
			PatientAction: PatientActionKeep,
		},
		PreventsMerge: !s.CanRun(),
	}

	sourcePatient, err := s.service.Get(ctx, s.source.Id.Hex(), *s.patient.UserId)
	if err != nil && !errors.Is(err, clinicians.ErrNotFound) {
		return err
	}
	if sourcePatient != nil {
		s.result.ReportDetails.PatientAction = PatientActionMergeInto
	}

	return nil
}

func (s *TargetPatientMergeTask) Run(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (s *TargetPatientMergeTask) GetResult() (TaskResult[PatientReportDetails], error) {
	return s.result, s.err
}

type SourcePatientMergeTask struct {
	patient patients.Patient

	source clinics.Clinic
	target clinics.Clinic

	service patients.Service

	result TaskResult[PatientReportDetails]
	err    error
}

func NewSourcePatientMergeTask(patient patients.Patient, source, target clinics.Clinic, service patients.Service) Task[PatientReportDetails] {
	return &SourcePatientMergeTask{
		patient: patient,
		source:  source,
		target:  target,
		service: service,
	}
}

func (s *SourcePatientMergeTask) CanRun() bool {
	return true
}

func (s *SourcePatientMergeTask) DryRun(ctx context.Context) error {
	s.result = TaskResult[PatientReportDetails]{
		ReportDetails: PatientReportDetails{
			PatientAction: PatientActionKeep,
		},
		PreventsMerge: !s.CanRun(),
	}

	//patients.Filter{
	//	Mrn:       nil,
	//	BirthDate: nil,
	//	FullName:  nil,
	//}
	//s.service.List(ctx, context.Context())

	return nil
}

func (s *SourcePatientMergeTask) Run(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (s *SourcePatientMergeTask) GetResult() (TaskResult[PatientReportDetails], error) {
	return s.result, s.err
}
