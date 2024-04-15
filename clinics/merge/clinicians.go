package merge

import (
	"context"
	"errors"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"sort"
)

const (
	// ClinicianActionKeep is used for target clinicians when there's no corresponding clinician in the source clinic
	ClinicianActionKeep = "KEEP"
	// ClinicianActionMerge is used when the source clinician will be merged to a target clinician record
	ClinicianActionMerge = "MERGE"
	// ClinicianActionMergeInto is when the target record will be the recipient of a merge
	ClinicianActionMergeInto = "MERGE_INTO"
	// ClinicianActionMove is used when the source clinician will be moved to the target clinic
	ClinicianActionMove = "MOVE"
)

type ClinicianReportDetails struct {
	ClinicianAction string
	Downgraded      bool
	Email           string
	Name            string
	ResultingRoles  []string
	Workspaces      []string
}

type SourceClinicianMergeTask struct {
	clinician clinicians.Clinician

	source clinics.Clinic
	target clinics.Clinic

	service clinicians.Service

	result TaskResult[ClinicianReportDetails]
	err    error
}

func NewSourceClinicianMergeTask(clinician clinicians.Clinician, source, target clinics.Clinic, service clinicians.Service) Task[ClinicianReportDetails] {
	return &SourceClinicianMergeTask{
		clinician: clinician,
		source:    source,
		target:    target,
		service:   service,
	}
}

func (s *SourceClinicianMergeTask) CanRun() bool {
	return true
}

func (s *SourceClinicianMergeTask) DryRun(ctx context.Context) error {
	s.result = TaskResult[ClinicianReportDetails]{
		ReportDetails: ClinicianReportDetails{
			ClinicianAction: ClinicianActionMove,
			Email:           *s.clinician.Email,
			Name:            *s.clinician.Name,
			ResultingRoles:  s.clinician.Roles,
			Workspaces:      []string{*s.source.Name},
		},
		PreventsMerge: !s.CanRun(),
	}

	targetClinician, err := s.service.Get(ctx, s.target.Id.Hex(), *s.clinician.UserId)
	if err != nil && !errors.Is(err, clinicians.ErrNotFound) {
		return err
	}
	if targetClinician != nil {
		s.result.ReportDetails.ClinicianAction = ClinicianActionMerge
		s.result.ReportDetails.Workspaces = append(s.result.ReportDetails.Workspaces, *s.target.Name)
		sort.Strings(s.result.ReportDetails.Workspaces)
		if s.clinician.IsAdmin() && !targetClinician.IsAdmin() {
			s.result.ReportDetails.Downgraded = true
			s.result.ReportDetails.ResultingRoles = targetClinician.Roles
		}
	}

	return nil
}

func (s *SourceClinicianMergeTask) Run(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (s *SourceClinicianMergeTask) GetResult() (TaskResult[ClinicianReportDetails], error) {
	return s.result, s.err
}

type TargetClinicianMergeTask struct {
	clinician clinicians.Clinician

	source clinics.Clinic
	target clinics.Clinic

	service clinicians.Service

	result TaskResult[ClinicianReportDetails]
	err    error
}

func NewTargetClinicianMergeTask(clinician clinicians.Clinician, source, target clinics.Clinic, service clinicians.Service) Task[ClinicianReportDetails] {
	return &TargetClinicianMergeTask{
		clinician: clinician,
		source:    source,
		target:    target,
		service:   service,
	}
}

func (s *TargetClinicianMergeTask) CanRun() bool {
	return true
}

func (s *TargetClinicianMergeTask) DryRun(ctx context.Context) error {
	s.result = TaskResult[ClinicianReportDetails]{
		ReportDetails: ClinicianReportDetails{
			ClinicianAction: ClinicianActionKeep,
			Email:           *s.clinician.Email,
			Name:            *s.clinician.Name,
			Workspaces:      []string{*s.target.Name},
		},
		PreventsMerge: !s.CanRun(),
	}

	sourceClinician, err := s.service.Get(ctx, s.target.Id.Hex(), *s.clinician.UserId)
	if err != nil && !errors.Is(err, clinicians.ErrNotFound) {
		return err
	}
	if sourceClinician != nil {
		s.result.ReportDetails.ClinicianAction = ClinicianActionMergeInto
	}

	return nil
}

func (s *TargetClinicianMergeTask) Run(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (s *TargetClinicianMergeTask) GetResult() (TaskResult[ClinicianReportDetails], error) {
	return s.result, s.err
}
