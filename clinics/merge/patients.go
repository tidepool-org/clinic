package merge

import (
	"context"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/tidepool-org/clinic/clinics"
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

func (p PatientPlan) HasConflicts() bool {
	for _, conflicts := range p.Conflicts {
		if len(conflicts) > 0 {
			return true
		}
	}
	return false
}

func (p PatientPlan) PreventsMerge() bool {
	return false
}

type Conflict struct {
	Category string
	Patient  patients.Patient
}

type PatientMergePlanner struct {
	source         clinics.Clinic
	target         clinics.Clinic
	sourcePatients []patients.Patient
	targetPatients []patients.Patient
	targetByUserId map[string]*patients.Patient
	sourceTags     map[string]*clinics.PatientTag
	targetTags     map[string]*clinics.PatientTag
}

func NewPatientMergePlanner(source, target clinics.Clinic, sourcePatients, targetPatients []patients.Patient) (*PatientMergePlanner, error) {
	planner := &PatientMergePlanner{
		source:         source,
		sourcePatients: sourcePatients,
		sourceTags:     buildTagsMap(source.PatientTags),
		target:         target,
		targetByUserId: make(map[string]*patients.Patient),
		targetPatients: targetPatients,
		targetTags:     buildTagsMap(target.PatientTags),
	}
	for _, patient := range targetPatients {
		planner.targetByUserId[getUserId(patient)] = &patient
	}
	return planner, nil
}

func (p *PatientMergePlanner) Plan(ctx context.Context) (PatientsPlan, error) {
	targetByAttribute := buildAttributeMap(p.targetPatients)
	mergeTargetPatients := map[string]struct{}{}
	list := make([]PatientPlan, 0, len(p.sourcePatients)+len(p.targetPatients))
	for _, patient := range p.sourcePatients {
		plan := PatientPlan{
			SourcePatient:  &patient,
			SourceTagNames: getPatientTagNames(patient, p.sourceTags),
			Conflicts:      make(map[string][]Conflict),
			PatientAction:  PatientActionMove,
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
				plan.TargetPatient = target
				plan.TargetTagNames = getPatientTagNames(*target, p.targetTags)

				uniqueTags := mapset.NewSet[string](plan.SourceTagNames...)
				uniqueTags.Append(plan.TargetTagNames...)
				plan.PostMigrationTagNames = uniqueTags.ToSlice()
			}
			plan.Conflicts[conflictCategory] = append(plan.Conflicts[conflictCategory], Conflict{
				Category: conflictCategory,
				Patient:  *target,
			})
		}
		list = append(list, plan)
	}

	for _, patient := range p.targetPatients {
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

func buildTagsMap(tags []clinics.PatientTag) map[string]*clinics.PatientTag {
	m := make(map[string]*clinics.PatientTag)
	for _, tag := range tags {
		m[tag.Id.Hex()] = &tag
	}
	return m
}

func getPatientTagNames(patient patients.Patient, tags map[string]*clinics.PatientTag) []string {
	if patient.Tags != nil && len(*patient.Tags) > 0 {
		tagNames := make([]string, 0, len(*patient.Tags))
		for _, tagId := range *patient.Tags {
			if tag, ok := tags[tagId.Hex()]; ok && tag != nil {
				tagNames = append(tagNames, tag.Name)
			}
		}
		return tagNames
	}
	return nil
}
