package merge

import (
	"context"
	"fmt"
	"slices"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/sites"
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

func (p PatientPlans) Errors() []ReportError {
	return PlansErrors(p)
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
	SourceClinicId *primitive.ObjectID `bson:"sourceClinicId"`
	TargetClinicId *primitive.ObjectID `bson:"targetClinicId"`

	SourceClinicName string `bson:"sourceClinicName"`
	TargetClinicName string `bson:"targetClinicName"`

	SourcePatient *patients.Patient `bson:"sourcePatient"`
	TargetPatient *patients.Patient `bson:"targetPatient"`

	Conflicts map[string][]Conflict `bson:"conflicts"`

	PatientAction string `bson:"patientAction"`

	SourceTagNames []string `bson:"sourceTagNames"`
	TargetTagNames []string `bson:"targetTagNames"`

	SourceSiteNames  []string `bson:"sourceSiteNames"`
	RenamedSiteNames []string `bson:"mergedSiteNames"`
	TargetSiteNames  []string `bson:"targetSiteNames"`

	PostMigrationTagNames      []string `bson:"postMigrationTagNames"`
	PostMigrationSiteNames     []string `bson:"postMigrationSiteNames"`
	PostMigrationMRNUniqueness bool     `bson:"postMigrationMRNUniqueness"`

	CanExecuteAction bool         `bson:"canExecuteAction"`
	Error            *ReportError `bson:"error"`
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
	return !p.CanExecuteAction || len(p.Errors()) > 0
}

func (p PatientPlan) Errors() []ReportError {
	if p.Error != nil {
		return []ReportError{*p.Error}
	}
	return nil
}

type Conflict struct {
	Category string           `bson:"category"`
	Patient  patients.Patient `bson:"patient"`
}

type PatientMergePlanner struct {
	source            clinics.Clinic
	target            clinics.Clinic
	sourceByAttribute attributeMap
	sourcePatients    []patients.Patient
	sourceTags        map[string]*clinics.PatientTag
	targetByAttribute attributeMap
	targetPatients    []patients.Patient
	targetTags        map[string]*clinics.PatientTag
	MergedSites       mergedSites
}

type mergedSites struct {
	ToMigrate []sites.Site
	ToRename  []sites.Site
	ToRetain  []sites.Site
}

func newMergedSites() *mergedSites {
	return &mergedSites{
		ToMigrate: []sites.Site{},
		ToRename:  []sites.Site{},
		ToRetain:  []sites.Site{},
	}
}

func NewPatientMergePlanner(source, target clinics.Clinic, sourcePatients, targetPatients []patients.Patient) (*PatientMergePlanner, error) {
	planner := &PatientMergePlanner{
		source:            source,
		sourceByAttribute: buildAttributeMap(sourcePatients),
		sourcePatients:    sourcePatients,
		sourceTags:        buildTagsMap(source.PatientTags),
		target:            target,
		targetByAttribute: buildAttributeMap(targetPatients),
		targetPatients:    targetPatients,
		targetTags:        buildTagsMap(target.PatientTags),
	}
	if err := planner.initMergedSites(); err != nil {
		return nil, err
	}
	return planner, nil
}

func (p *PatientMergePlanner) initMergedSites() error {
	result := newMergedSites()
	targetSiteNames := map[string]struct{}{}
	for _, targetSite := range p.target.Sites {
		if _, found := targetSiteNames[targetSite.Name]; !found {
			result.ToRetain = append(result.ToRetain, targetSite)
		} else {
			msg := "found duplicate target site, that shouldn't happen: %s"
			return fmt.Errorf(msg, targetSite.Name)
		}
	}
	for _, sourceSite := range p.source.Sites {
		if _, found := targetSiteNames[sourceSite.Name]; !found {
			result.ToMigrate = append(result.ToMigrate, sourceSite)
		} else {
			result.ToRename = append(result.ToRename, sourceSite)
		}
	}
	p.MergedSites = *result
	return nil
}

func siteNames(sites *[]sites.Site, targetSites []sites.Site) []string {
	if sites == nil {
		return nil
	}
	names := make([]string, 0, len(*sites))
	for _, site := range *sites {
		renamed, err := maybeRenameSite(site, targetSites)
		if err != nil {
			renamed = fmt.Sprintf("<error incrementing site name: %s>", err)
		}
		names = append(names, renamed)
	}
	slices.Sort(names)
	return names
}

func (p *PatientMergePlanner) Plan(ctx context.Context) (PatientPlans, error) {
	mergeTargetPatients := map[string]struct{}{}
	list := make([]PatientPlan, 0, len(p.sourcePatients)+len(p.targetPatients))
	for _, patient := range p.sourcePatients {
		plan := PatientPlan{
			SourceClinicId:         p.source.Id,
			SourceClinicName:       *p.source.Name,
			TargetClinicId:         p.target.Id,
			TargetClinicName:       *p.target.Name,
			SourcePatient:          &patient,
			SourceTagNames:         getUniquePatientTagNames(patient, p.sourceTags),
			SourceSiteNames:        siteNames(patient.Sites, []sites.Site{}),
			RenamedSiteNames:       siteNames(patient.Sites, p.target.Sites),
			Conflicts:              make(map[string][]Conflict),
			PatientAction:          PatientActionMove,
			PostMigrationTagNames:  getUniquePatientTagNames(patient, p.sourceTags),
			PostMigrationSiteNames: siteNames(patient.Sites, p.target.Sites),
			CanExecuteAction:       true,
		}

		duplicates := getDuplicates(patient, p.targetByAttribute)
		for userId, conflictCategory := range duplicates {
			target, err := p.getTargetPatientById(userId)
			if err != nil {
				return nil, err
			}

			if conflictCategory == PatientConflictCategoryDuplicateAccounts {
				mergeTargetPatients[userId] = struct{}{}
				plan.PatientAction = PatientActionMerge
				plan.TargetPatient = target
				plan.TargetTagNames = getUniquePatientTagNames(*target, p.targetTags)
				plan.TargetSiteNames = siteNames(target.Sites, p.target.Sites)

				uniqueTags := mapset.NewSet(plan.SourceTagNames...)
				uniqueTags.Append(plan.TargetTagNames...)
				plan.PostMigrationTagNames = uniqueTags.ToSlice()
				if patient.Sites == nil || target == nil || target.Sites == nil {
					return nil, fmt.Errorf("unable to combine sites for duplicate patients")
				}
				combinedSites := slices.Concat(*target.Sites, *patient.Sites)
				plan.PostMigrationSiteNames = siteNames(&combinedSites, *target.Sites)
			}
			plan.Conflicts[conflictCategory] = append(plan.Conflicts[conflictCategory], Conflict{
				Category: conflictCategory,
				Patient:  *target,
			})
		}
		if plan.PatientAction == PatientActionMove {
			if p.target.MRNSettings != nil {
				if mrn := getMRN(patient); mrn == "" {
					// Do not allow moving patients without MRNs to clinics where MRNs are required
					if p.target.MRNSettings.Required {
						plan.CanExecuteAction = false
						plan.Error = &ErrorMRNRequiredInTargetWorkspace
					}
				} else {
					if p.target.MRNSettings.Unique {
						// Ensure MRNs are unique after patients are moved
						plan.PostMigrationMRNUniqueness = true

						// Do not allow moving patients if there are patients with the same MRN in the target clinic
						if pts := p.targetByAttribute.GetPatientsWithMRN(mrn); len(pts) > 0 {
							plan.CanExecuteAction = false
							plan.Error = &ErrorDuplicateMRNInTargetWorkspace
						}

						// Do not allow moving patients if there are patients with the same MRN in the source clinic
						if pts := p.sourceByAttribute.GetPatientsWithMRN(mrn); len(pts) > 1 {
							plan.CanExecuteAction = false
							plan.Error = &ErrorDuplicateMRNInSourceWorkspace
						}
					}
				}
			}
		}
		list = append(list, plan)
	}

	for _, patient := range p.targetPatients {
		plan := PatientPlan{
			SourceClinicId:   p.source.Id,
			SourceClinicName: *p.source.Name,
			TargetClinicId:   p.target.Id,
			TargetClinicName: *p.target.Name,
			TargetPatient:    &patient,
			TargetTagNames:   getUniquePatientTagNames(patient, p.targetTags),
			TargetSiteNames:  siteNames(patient.Sites, p.target.Sites),
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
	pts := p.targetByAttribute[PatientAttributeUserId][userId]
	if len(pts) == 0 || pts[0] == nil {
		return nil, fmt.Errorf("target patient with id %s doesn't exist", userId)
	}
	if len(pts) > 1 {
		return nil, fmt.Errorf("found multiple patients with user id %s", userId)
	}
	return pts[0], nil
}

func buildTagsMap(tags []clinics.PatientTag) map[string]*clinics.PatientTag {
	m := make(map[string]*clinics.PatientTag)
	for _, tag := range tags {
		m[tag.Id.Hex()] = &tag
	}
	return m
}

func getUniquePatientTagNames(patient patients.Patient, tags map[string]*clinics.PatientTag) []string {
	if patient.Tags != nil && len(*patient.Tags) > 0 {
		tagNames := mapset.NewSet[string]()
		for _, tagId := range *patient.Tags {
			if tag, ok := tags[tagId.Hex()]; ok && tag != nil {
				tagNames.Append(tag.Name)
			}
		}
		return tagNames.ToSlice()
	}
	return nil
}

// Do not persist summaries
func sanitizePatient(patient *patients.Patient) {
	patient.Summary = nil
}

type PatientPlanExecutor struct {
	clinicsService     clinics.Service
	patientsCollection *mongo.Collection

	logger *zap.SugaredLogger
}

func NewPatientPlanExecutor(logger *zap.SugaredLogger, clinicsService clinics.Service, db *mongo.Database) *PatientPlanExecutor {
	return &PatientPlanExecutor{
		clinicsService:     clinicsService,
		patientsCollection: db.Collection(patients.CollectionName),

		logger: logger,
	}
}

func (p *PatientPlanExecutor) Execute(ctx context.Context, plan PatientPlan, source, target clinics.Clinic) error {
	// Fetch the updated clinic object to make sure we are capturing
	// the tags that were migrated from the source clinic
	updated, err := p.clinicsService.Get(ctx, target.Id.Hex())
	if err != nil {
		return err
	}

	switch plan.PatientAction {
	case PatientActionMove:
		p.logger.Infow(
			"moving patient",
			"clinicId", source.Id.Hex(),
			"userId", plan.SourcePatient.UserId,
			"targetClinicId", target.Id.Hex(),
		)
		return p.movePatient(ctx, plan, *updated)
	case PatientActionMerge:
		p.logger.Infow(
			"merging patient",
			"clinicId", source.Id.Hex(),
			"userId", plan.SourcePatient.UserId,
			"targetClinicId", target.Id.Hex(),
			"targetUserId", *plan.TargetPatient.UserId,
		)
		return p.mergePatient(ctx, plan, *updated)
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
			"clinicId":         plan.TargetClinicId,
			"requireUniqueMrn": plan.PostMigrationMRNUniqueness,
			"tags":             tagIds,
			"updatedTime":      time.Now(),
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
	if err := p.mergeSites(ctx, plan, target); err != nil {
		return err
	}
	if err := p.deleteSourcePatient(ctx, plan); err != nil {
		return fmt.Errorf("error deleting patient %s: %w", *plan.SourcePatient.UserId, err)
	}
	return nil
}

func (p *PatientPlanExecutor) mergeSites(ctx context.Context, plan PatientPlan, target clinics.Clinic) error {
	selector := bson.M{
		"clinicId": plan.TargetPatient.ClinicId,
		"userId":   plan.TargetPatient.UserId,
	}
	if plan.SourcePatient != nil && plan.SourcePatient.Sites != nil {
		srcSites := plan.SourcePatient.Sites
		tgtSites := target.Sites
		for i, srcSite := range *srcSites {
			for _, tgtSite := range tgtSites {
				if srcSite.Id == tgtSite.Id && srcSite.Name != tgtSite.Name {
					(*srcSites)[i].Name = tgtSite.Name
				}
			}
		}
	}
	update := bson.M{
		"$push": bson.M{
			"sites": bson.M{
				"$each": plan.SourcePatient.Sites,
			},
		},
		"$currentDate": bson.M{"updatedTime": true},
	}
	res, err := p.patientsCollection.UpdateOne(ctx, selector, update)
	if err != nil {
		return fmt.Errorf("error merging patient sites: %w", err)
	}
	if res.ModifiedCount != 1 {
		return fmt.Errorf("error updating patient %s sites: unexpected modified count %d", selector, res.ModifiedCount)
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
