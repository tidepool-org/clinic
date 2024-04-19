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

type ClinicianPlan struct {
	ClinicianAction string
	Downgraded      bool
	Email           string
	Name            string
	ResultingRoles  []string
	Workspaces      []string
}

func (c ClinicianPlan) PreventsMerge() bool {
	return false
}

type SourceClinicianMergePlanner struct {
	clinician clinicians.Clinician

	source clinics.Clinic
	target clinics.Clinic

	service clinicians.Service
}

func NewSourceClinicianMergePlanner(clinician clinicians.Clinician, source, target clinics.Clinic, service clinicians.Service) Planner[ClinicianPlan] {
	return &SourceClinicianMergePlanner{
		clinician: clinician,
		source:    source,
		target:    target,
		service:   service,
	}
}

func (s *SourceClinicianMergePlanner) Plan(ctx context.Context) (ClinicianPlan, error) {
	plan := ClinicianPlan{
		ClinicianAction: ClinicianActionMove,
		Email:           *s.clinician.Email,
		Name:            *s.clinician.Name,
		ResultingRoles:  s.clinician.Roles,
		Workspaces:      []string{*s.source.Name},
	}

	targetClinician, err := s.service.Get(ctx, s.target.Id.Hex(), *s.clinician.UserId)
	if err != nil && !errors.Is(err, clinicians.ErrNotFound) {
		return plan, err
	}
	if targetClinician != nil {
		plan.ClinicianAction = ClinicianActionMerge
		plan.Workspaces = append(plan.Workspaces, *s.target.Name)
		sort.Strings(plan.Workspaces)
		if s.clinician.IsAdmin() && !targetClinician.IsAdmin() {
			plan.Downgraded = true
			plan.ResultingRoles = targetClinician.Roles
		}
	}

	return plan, nil
}

type TargetClinicianMergePlanner struct {
	clinician clinicians.Clinician

	source clinics.Clinic
	target clinics.Clinic

	service clinicians.Service
}

func NewTargetClinicianMergePlanner(clinician clinicians.Clinician, source, target clinics.Clinic, service clinicians.Service) Planner[ClinicianPlan] {
	return &TargetClinicianMergePlanner{
		clinician: clinician,
		source:    source,
		target:    target,
		service:   service,
	}
}

func (s *TargetClinicianMergePlanner) CanRun() bool {
	return true
}

func (s *TargetClinicianMergePlanner) Plan(ctx context.Context) (ClinicianPlan, error) {
	plan := ClinicianPlan{
		ClinicianAction: ClinicianActionKeep,
		Email:           *s.clinician.Email,
		Name:            *s.clinician.Name,
		Workspaces:      []string{*s.target.Name},
	}

	sourceClinician, err := s.service.Get(ctx, s.target.Id.Hex(), *s.clinician.UserId)
	if err != nil && !errors.Is(err, clinicians.ErrNotFound) {
		return plan, err
	}
	if sourceClinician != nil {
		plan.ClinicianAction = ClinicianActionMergeInto
	}

	return plan, nil
}
