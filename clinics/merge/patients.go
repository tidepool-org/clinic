package merge

import (
	"context"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"time"
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

type PatientPlans []PatientPlan

func (p PatientPlans) PreventsMerge() bool {
	return PlansPreventMerge(p)
}

func (p PatientPlans) GetSourcePatientPlans() PatientPlans {
	var plans PatientPlans
	for _, plan := range p {
		if plan.SourcePatient != nil {
			plans = append(plans, plan)
		}
	}
	return plans
}

func (p PatientPlans) GetTargetPatientPlans() PatientPlans {
	var plans PatientPlans
	for _, plan := range p {
		if plan.SourcePatient == nil && plan.TargetPatient != nil {
			plans = append(plans, plan)
		}
	}
	return plans
}

func (p PatientPlans) GetResultingPatientsCount() int {
	count := 0
	for _, plan := range p {
		if plan.PatientAction != PatientActionMergeInto {
			count++
		}
	}
	return count
}

func (p PatientPlans) GetConflictCounts() map[string]int {
	result := make(map[string]int)

	for _, plan := range p {
		if plan.PatientAction == PatientActionMergeInto {
			continue
		}

		for _, conflicts := range plan.Conflicts {
			for _, conflict := range conflicts {
				count := result[conflict.Category]
				count++
				result[conflict.Category] = count
			}
		}
	}

	return result
}

type PatientPlan struct {
	SourceClinicId *primitive.ObjectID
	TargetClinicId *primitive.ObjectID

	SourcePatient *patients.Patient
	TargetPatient *patients.Patient

	Conflicts map[string][]Conflict

	PatientAction string

	SourceTagNames []string
	TargetTagNames []string

	PostMigrationTagNames []string

	CanExecuteAction bool
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
	return !p.CanExecuteAction
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

func (p *PatientMergePlanner) Plan(ctx context.Context) (PatientPlans, error) {
	targetByAttribute := buildAttributeMap(p.targetPatients)
	mergeTargetPatients := map[string]struct{}{}
	list := make([]PatientPlan, 0, len(p.sourcePatients)+len(p.targetPatients))
	for _, patient := range p.sourcePatients {
		plan := PatientPlan{
			SourceClinicId:   p.source.Id,
			TargetClinicId:   p.target.Id,
			SourcePatient:    &patient,
			SourceTagNames:   getPatientTagNames(patient, p.sourceTags),
			Conflicts:        make(map[string][]Conflict),
			PatientAction:    PatientActionMove,
			CanExecuteAction: true,
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
		if plan.PatientAction == PatientActionMove {
			// Do not allow moving patients without MRNs to clinics where MRNs are required
			if p.target.MRNSettings != nil && p.target.MRNSettings.Required && (patient.Mrn == nil || *patient.Mrn == "") {
				plan.CanExecuteAction = false
			}
		}
		list = append(list, plan)
	}

	for _, patient := range p.targetPatients {
		plan := PatientPlan{
			SourceClinicId:   p.source.Id,
			TargetClinicId:   p.target.Id,
			TargetPatient:    &patient,
			TargetTagNames:   getPatientTagNames(patient, p.targetTags),
			CanExecuteAction: true,
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

type PatientPlanExecutor struct {
	logger             *zap.SugaredLogger
	patientsCollection *mongo.Collection
}

func NewPatientPlanExecutor(logger *zap.SugaredLogger, db *mongo.Database) *PatientPlanExecutor {
	return &PatientPlanExecutor{
		logger:             logger,
		patientsCollection: db.Collection(patients.CollectionName),
	}
}

func (p *PatientPlanExecutor) Execute(ctx context.Context, plan PatientPlan, source, target clinics.Clinic) error {
	switch plan.PatientAction {
	case PatientActionMove:
		p.logger.Infow(
			"moving patient",
			"clinicId", source.Id.Hex(),
			"userId", plan.SourcePatient.UserId,
			"targetClinicId", target.Id.Hex(),
		)
		return p.movePatient(ctx, plan, target)
	case PatientActionMerge:
		p.logger.Infow(
			"merging patient",
			"clinicId", source.Id.Hex(),
			"userId", plan.SourcePatient.UserId,
			"targetClinicId", target.Id.Hex(),
			"targetUserId", *plan.TargetPatient.UserId,
		)
		return p.mergePatient(ctx, plan, target)
	case PatientActionRetain:
		p.logger.Infow(
			"retaining patient",
			"clinicId", target.Id.Hex(),
			"userId", plan.TargetPatient.UserId,
		)
		return nil
	case PatientActionMergeInto:
		p.logger.Infow(
			"patient is a target of a merge",
			"clinicId", target.Id.Hex(),
			"userId", plan.TargetPatient.UserId,
		)
		return nil
	default:
		return fmt.Errorf("unexpected plan action %s", plan.PatientAction)
	}
}

func (p *PatientPlanExecutor) movePatient(ctx context.Context, plan PatientPlan, target clinics.Clinic) error {
	tagNames := map[string]struct{}{}
	for _, name := range plan.PostMigrationTagNames {
		tagNames[name] = struct{}{}
	}

	tagIds := make([]primitive.ObjectID, 0, len(tagNames))
	for _, tag := range target.PatientTags {
		if _, ok := tagNames[tag.Name]; ok {
			tagIds = append(tagIds, *tag.Id)
		}
	}

	selector := bson.M{
		"clinicId": plan.SourcePatient.ClinicId,
		"userId":   plan.SourcePatient.UserId,
	}

	update := bson.M{
		"$set": bson.M{
			"clinicId":    plan.TargetClinicId,
			"tags":        tagIds,
			"updatedTime": time.Now(),
		},
	}

	res, err := p.patientsCollection.UpdateOne(ctx, selector, update)
	if err != nil {
		return fmt.Errorf("error moving patient: %w", err)
	}
	if res.ModifiedCount != 1 {
		return fmt.Errorf("error moving patient: unexpected modified count %v", res.ModifiedCount)
	}
	return nil
}

func (p *PatientPlanExecutor) mergePatient(ctx context.Context, plan PatientPlan, target clinics.Clinic) error {
	if err := p.mergeTags(ctx, plan, target); err != nil {
		return fmt.Errorf("error updating patient tags: %w", err)
	}
	if err := p.deleteSourcePatient(ctx, plan); err != nil {
		return fmt.Errorf("error deleting patient %s: %w", *plan.SourcePatient.UserId, err)
	}
	return nil
}

func (p *PatientPlanExecutor) mergeTags(ctx context.Context, plan PatientPlan, target clinics.Clinic) error {
	tagNames := map[string]struct{}{}
	for _, name := range plan.PostMigrationTagNames {
		tagNames[name] = struct{}{}
	}
	tagIds := make([]primitive.ObjectID, 0, len(tagNames))
	for _, tag := range target.PatientTags {
		if _, ok := tagNames[tag.Name]; ok {
			tagIds = append(tagIds, *tag.Id)
			delete(tagNames, tag.Name)
		}
	}
	if len(tagIds) == 0 {
		return nil
	}

	selector := bson.M{
		"clinicId": plan.TargetPatient.ClinicId,
		"userId":   plan.TargetPatient.UserId,
	}
	update := bson.M{
		"$set": bson.M{
			"tags":        tagIds,
			"updatedTime": time.Now(),
		},
	}

	res, err := p.patientsCollection.UpdateOne(ctx, selector, update)
	if err != nil {
		return fmt.Errorf("error updating patient %s tags: %w", selector, err)
	}
	if res.ModifiedCount != 1 {
		return fmt.Errorf("error updating patient %s tags: unexpected modified count %v", selector, res.ModifiedCount)
	}

	return nil
}

func (p *PatientPlanExecutor) deleteSourcePatient(ctx context.Context, plan PatientPlan) error {
	selector := bson.M{
		"clinicId": plan.SourcePatient.ClinicId,
		"userId":   plan.SourcePatient.UserId,
	}
	res, err := p.patientsCollection.DeleteOne(ctx, selector)
	if err != nil {
		return err
	}
	if res.DeletedCount != 1 {
		return fmt.Errorf("unexpected deleted count %v", res.DeletedCount)
	}
	return nil
}
