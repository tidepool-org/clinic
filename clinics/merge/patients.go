package merge

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"golang.org/x/sync/errgroup"
	"math"
	"slices"
	"sync"
)

const (
	PatientConflictCategoryDuplicateAccounts         = "Duplicate Accounts"
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

	PatientDuplicateAttributeUserId   = "USER_ID"
	PatientDuplicateAttributeMRN      = "MRN"
	PatientDuplicateAttributeDOB      = "DOB"
	PatientDuplicateAttributeFullName = "FULL_NAME"
)

var (
	ConflictRankMap = map[string]int{
		PatientConflictCategoryDuplicateAccounts:         0,
		PatientConflictCategoryLikelyDuplicateAccounts:   1,
		PatientConflictCategoryPossibleDuplicateAccounts: 2,
		PatientConflictCategoryDuplicateMRNs:             3,
	}
)

type PatientPlan struct {
	SourcePatient patients.Patient
	TargetPatient patients.Patient

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
	Category            string
	DuplicateAttributes []string
	Patient             patients.Patient
}

type TargetPatientMergePlanner struct {
	patient patients.Patient

	source clinics.Clinic
	target clinics.Clinic

	service patients.Service
}

type PatientPlannerFactoryFn func(patient patients.Patient, source, target clinics.Clinic, service patients.Service) Planner[PatientPlan]

func NewTargetPatientMergePlanner(patient patients.Patient, source, target clinics.Clinic, service patients.Service) Planner[PatientPlan] {
	return &TargetPatientMergePlanner{
		patient: patient,
		source:  source,
		target:  target,
		service: service,
	}
}

func (s *TargetPatientMergePlanner) Plan(ctx context.Context) (PatientPlan, error) {
	plan := PatientPlan{
		PatientAction: PatientActionKeep,
	}

	sourcePatient, err := s.service.Get(ctx, s.source.Id.Hex(), *s.patient.UserId)
	if err != nil && !errors.Is(err, clinicians.ErrNotFound) {
		return plan, err
	}
	if sourcePatient != nil {
		plan.PatientAction = PatientActionMergeInto
	}

	return plan, nil
}

type SourcePatientMergePlanner struct {
	patient patients.Patient

	source clinics.Clinic
	target clinics.Clinic

	service patients.Service
}

func NewSourcePatientMergePlanner(patient patients.Patient, source, target clinics.Clinic, service patients.Service) Planner[PatientPlan] {
	return &SourcePatientMergePlanner{
		patient: patient,
		source:  source,
		target:  target,
		service: service,
	}
}

func (s *SourcePatientMergePlanner) Plan(ctx context.Context) (PatientPlan, error) {
	plan := PatientPlan{
		SourcePatient: s.patient,
		PatientAction: PatientActionKeep,
		Conflicts:     make(map[string][]Conflict),
	}

	matching, err := s.getMatchingPatients(ctx)
	if err != nil {
		return plan, err
	}

	conflicts := make([]Conflict, 0, len(matching))
	for _, p := range matching {
		if conflict := s.processMatchingPatient(p); conflict.Category != "" {
			conflicts = append(conflicts, conflict)
		}
	}
	slices.SortFunc(conflicts, sortConflicts)

	if conflicts, ok := plan.Conflicts[PatientConflictCategoryDuplicateAccounts]; ok && len(conflicts) > 0 {
		plan.PatientAction = PatientActionMerge

		uniqueTagNames := map[string]struct{}{}

		sourceClinicTags := map[string]clinics.PatientTag{}
		for _, tag := range s.source.PatientTags {
			sourceClinicTags[tag.Id.Hex()] = tag
			plan.SourceTagNames = append(plan.SourceTagNames, tag.Name)
		}
		if s.patient.Tags != nil {
			for _, tagRef := range *s.patient.Tags {
				if tag, ok := sourceClinicTags[tagRef.Hex()]; ok {
					uniqueTagNames[tag.Name] = struct{}{}
				}
			}
		}

		targetPatient := conflicts[0].Patient
		targetClinicTags := map[string]clinics.PatientTag{}
		for _, tag := range s.target.PatientTags {
			targetClinicTags[tag.Id.Hex()] = tag
			plan.TargetTagNames = append(plan.TargetTagNames, tag.Name)
		}
		if targetPatient.Tags != nil {
			for _, tagRef := range *targetPatient.Tags {
				if tag, ok := targetClinicTags[tagRef.Hex()]; ok {
					uniqueTagNames[tag.Name] = struct{}{}
				}
			}
		}

		tagNames := make([]string, 0, len(uniqueTagNames))
		for tagName := range uniqueTagNames {
			tagNames = append(tagNames, tagName)
		}

		plan.PostMigrationTagNames = tagNames
	} else {
		plan.PatientAction = PatientActionMove
		plan.PostMigrationMRN = s.patient.Mrn
		plan.PostMigrationFullName = s.patient.FullName

		for _, conflicts := range plan.Conflicts {
			for _, conflict := range conflicts {
				for _, attr := range conflict.DuplicateAttributes {
					if attr == PatientDuplicateAttributeMRN && s.patient.Mrn != nil {
						mrn := fmt.Sprintf("DUPE_%s", *s.patient.Mrn)
						plan.PostMigrationMRN = &mrn
					} else if attr == PatientDuplicateAttributeFullName && s.patient.FullName != nil {
						fullName := fmt.Sprintf("%s (2)", *s.patient.FullName)
						plan.PostMigrationFullName = &fullName
					}
				}
			}
		}
	}

	return plan, nil
}

func (s *SourcePatientMergePlanner) processMatchingPatient(match patients.Patient) Conflict {
	duplicateAttributes := map[string]struct{}{}
	if s.patient.UserId != nil && match.UserId != nil && *s.patient.UserId == *match.UserId {
		duplicateAttributes[PatientDuplicateAttributeUserId] = struct{}{}
	}
	if s.patient.Mrn != nil && match.Mrn != nil && *s.patient.Mrn == *match.Mrn {
		duplicateAttributes[PatientDuplicateAttributeMRN] = struct{}{}
	}
	if s.patient.BirthDate != nil && match.BirthDate != nil && *s.patient.BirthDate == *match.BirthDate {
		duplicateAttributes[PatientDuplicateAttributeDOB] = struct{}{}
	}
	if s.patient.FullName != nil && match.FullName != nil && *s.patient.FullName == *match.FullName {
		duplicateAttributes[PatientDuplicateAttributeFullName] = struct{}{}
	}

	conflictCategory := ""
	if _, ok := duplicateAttributes[PatientDuplicateAttributeUserId]; ok {
		conflictCategory = PatientConflictCategoryDuplicateAccounts
	} else if len(duplicateAttributes) >= 2 {
		conflictCategory = PatientConflictCategoryLikelyDuplicateAccounts
	} else if len(duplicateAttributes) == 1 {
		if _, ok := duplicateAttributes[PatientDuplicateAttributeFullName]; ok {
			conflictCategory = PatientConflictCategoryPossibleDuplicateAccounts
		} else if _, ok = duplicateAttributes[PatientDuplicateAttributeMRN]; ok {
			conflictCategory = PatientConflictCategoryDuplicateMRNs
		}
	}

	attrs := make([]string, 0, len(duplicateAttributes))
	for attr := range duplicateAttributes {
		attrs = append(attrs, attr)
	}

	return Conflict{
		Category:            conflictCategory,
		DuplicateAttributes: attrs,
		Patient:             match,
	}
}

func (s *SourcePatientMergePlanner) getMatchingPatients(ctx context.Context) ([]patients.Patient, error) {
	patientsLimit := 100
	filters := s.getFilters()
	if len(filters) == 0 {
		return nil, nil
	}

	result := make([]patients.Patient, 0)
	uniqueIds := map[string]struct{}{}
	mu := &sync.Mutex{}

	g, ctx := errgroup.WithContext(ctx)
	for _, filter := range filters {
		page := store.DefaultPagination().WithLimit(patientsLimit)
		g.Go(func() error {
			list, err := s.service.List(ctx, &filter, page, nil)
			if err != nil {
				return err
			}
			if list.TotalCount > patientsLimit {
				return fmt.Errorf(
					"found too many matching patients (%v) in target clinic %v for patient %v",
					list.TotalCount,
					s.target.Id.Hex(),
					*s.patient.UserId,
				)
			}

			mu.Lock()
			defer mu.Unlock()

			// Only add unique patients
			for _, p := range list.Patients {
				if p != nil && p.UserId != nil {
					if _, ok := uniqueIds[*p.UserId]; !ok {
						uniqueIds[*p.UserId] = struct{}{}
						result = append(result, *p)
					}
				}
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *SourcePatientMergePlanner) getFilters() []patients.Filter {
	targetClinicId := s.target.Id.Hex()
	filters := make([]patients.Filter, 0, 4)
	if s.patient.UserId != nil {
		filters = append(filters, patients.Filter{
			ClinicId: &targetClinicId,
			UserId:   s.patient.UserId,
		})
	}
	if s.patient.Mrn != nil {
		filters = append(filters, patients.Filter{
			ClinicId: &targetClinicId,
			Mrn:      s.patient.Mrn,
		})
	}
	if s.patient.BirthDate != nil {
		filters = append(filters, patients.Filter{
			ClinicId:  &targetClinicId,
			BirthDate: s.patient.BirthDate,
		})
	}
	if s.patient.FullName != nil {
		filters = append(filters, patients.Filter{
			ClinicId: &targetClinicId,
			FullName: s.patient.FullName,
		})
	}
	return filters
}

func sortConflicts(a, b Conflict) int {
	res := cmp.Compare(getCategoryRank(a.Category), getCategoryRank(b.Category))
	if res == 0 {
		res = cmp.Compare(len(a.DuplicateAttributes), len(b.DuplicateAttributes))
	}
	if res == 0 {
		aName := ""
		bName := ""
		if a.Patient.FullName != nil {
			aName = *a.Patient.FullName
		}
		if b.Patient.FullName != nil {
			bName = *b.Patient.FullName
		}
		res = cmp.Compare(aName, bName)
	}
	return res
}
func getCategoryRank(category string) int {
	if v, ok := ConflictRankMap[category]; ok {
		return v
	}

	return math.MaxInt
}
