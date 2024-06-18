package merge

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/patients"
)

const (
	PatientConflictCategoryDuplicateAccounts       = "Duplicate Accounts"
	PatientConflictCategoryLikelyDuplicateAccounts = "Likely Duplicate Accounts"
	PatientConflictCategoryMRNOnlyMatch            = "MRN Only Match"
	PatientConflictCategoryNameOnlyMatch           = "Name Only Match"

	// PatientActionRetain is used for target patients when there's no corresponding patient in the source clinic
	PatientActionRetain = "RETAIN"
	// PatientActionMerge is used when the source patient will be merged to a target patient record
	PatientActionMerge = "MERGE"
	// PatientActionMergeInto is when a patient record in the target clinic will be a target of a merge
	PatientActionMergeInto = "MERGE_INTO"
	// PatientActionMove is used when the source patient will be moved to the target clinic
	PatientActionMove = "MOVE"
)

type PatientsPlan []PatientPlan

func (p PatientsPlan) PreventsMerge() bool {
	for _, plan := range p {
		if plan.PreventsMerge() {
			return true
		}
	}
	return false
}

type PatientPlan struct {
	SourcePatient *patients.Patient
	TargetPatient *patients.Patient

	Conflicts map[string][]Conflict

	PatientAction string

	SourceTagNames []string
	TargetTagNames []string

	PostMigrationMRN      *string
	PostMigrationFullName *string
	PostMigrationTagNames []string
}

func (p PatientPlan) PreventsMerge() bool {
	return false
}

type Conflict struct {
	Category string
	Patient  patients.Patient
}

type PatientMergePlanner struct {
	source         []patients.Patient
	target         []patients.Patient
	targetByUserId map[string]*patients.Patient
}

func NewPatientMergePlanner(source, target []patients.Patient) (*PatientMergePlanner, error) {
	planner := &PatientMergePlanner{
		source:         source,
		target:         target,
		targetByUserId: make(map[string]*patients.Patient),
	}
	for _, patient := range target {
		planner.targetByUserId[getUserId(patient)] = &patient
	}
	return planner, nil
}

func (p *PatientMergePlanner) Plan(ctx context.Context) (PatientsPlan, error) {
	targetByAttribute := buildAttributeMap(p.target)
	mergeTargetPatients := map[string]struct{}{}
	list := make([]PatientPlan, 0, len(p.source)+len(p.target))
	for _, patient := range p.source {
		plan := PatientPlan{
			SourcePatient: &patient,
			Conflicts:     make(map[string][]Conflict),
			PatientAction: PatientActionMove,
		}
		duplicates := getDuplicates(patient, targetByAttribute)
		for userId, conflictCategory := range duplicates {
			target, err := p.getTargetPatientById(userId)
			if err != nil {
				return nil, err
			}

			if conflictCategory == PatientConflictCategoryDuplicateAccounts {
				mergeTargetPatients[userId] = struct{}{}
				plan.PatientAction = PatientActionMerge
				// TODO: Set resulting attributes
			}
			plan.Conflicts[conflictCategory] = append(plan.Conflicts[conflictCategory], Conflict{
				Category: conflictCategory,
				Patient:  *target,
			})
		}
		list = append(list, plan)
	}

	for _, patient := range p.target {
		plan := PatientPlan{
			TargetPatient: &patient,
		}
		if _, ok := mergeTargetPatients[getUserId(patient)]; ok {
			plan.PatientAction = PatientActionMergeInto
		} else {
			plan.PatientAction = PatientActionRetain
		}
		list = append(list, plan)
	}

	return list, nil
}

func (p *PatientMergePlanner) getTargetPatientById(userId string) (*patients.Patient, error) {
	patient, ok := p.targetByUserId[userId]
	if !ok || patient == nil {
		return nil, fmt.Errorf("target patient with id %s doesn't exist", userId)
	}
	return patient, nil
}
