package merge

import (
	"context"
	"errors"
	"fmt"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"sort"
	"time"
)

const (
	// ClinicianActionRetain is used for target clinicians when there's no corresponding clinician in the source clinic
	ClinicianActionRetain = "RETAIN"
	// ClinicianActionMerge is used when the source clinician will be merged to a target clinician record
	ClinicianActionMerge = "MERGE"
	// ClinicianActionMergeInto is when the target record will be the recipient of a merge
	ClinicianActionMergeInto = "MERGE_INTO"
	// ClinicianActionMove is used when the source clinician will be moved to the target clinic
	ClinicianActionMove = "MOVE"
)

type ClinicianPlan struct {
	Clinician       clinicians.Clinician
	ClinicianAction string
	Downgraded      bool
	ResultingRoles  []string
	Workspaces      []string
}

func (c ClinicianPlan) IsPendingInvite() bool {
	return c.Clinician.UserId == nil || *c.Clinician.UserId == ""
}

func (c ClinicianPlan) PreventsMerge() bool {
	return c.ClinicianAction == ClinicianActionMove && c.IsPendingInvite()
}

func (c ClinicianPlan) GetClinicianName() string {
	if c.Clinician.Name == nil {
		return ""
	}
	return *c.Clinician.Name
}

func (c ClinicianPlan) GetClinicianEmail() string {
	if c.Clinician.Email == nil {
		return ""
	}
	return *c.Clinician.Email
}

type ClinicianPlans []ClinicianPlan

func (c ClinicianPlans) PreventsMerge() bool {
	return PlansPreventMerge(c)
}

func (c ClinicianPlans) PendingInvitesByWorkspace() map[string]int{
	result := make(map[string]int)
	for _, p := range c {
		if p.IsPendingInvite() {
			result[p.Workspaces[0]] = result[p.Workspaces[0]] + 1
		}
	}
	return result
}

func (c ClinicianPlans) GetDowngradedMembersCount() int {
	count := 0
	for _, p := range c {
		if p.Downgraded {
			count++
		}
	}
	return count
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
		Clinician:       s.clinician,
		ClinicianAction: ClinicianActionMove,
		ResultingRoles:  s.clinician.Roles,
		Workspaces:      []string{*s.source.Name},
	}

	if s.clinician.UserId != nil {
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

func (s *TargetClinicianMergePlanner) Plan(ctx context.Context) (ClinicianPlan, error) {
	plan := ClinicianPlan{
		Clinician:       s.clinician,
		ClinicianAction: ClinicianActionRetain,
		ResultingRoles:  s.clinician.Roles,
		Workspaces:      []string{*s.target.Name},
	}

	if s.clinician.UserId != nil {
		sourceClinician, err := s.service.Get(ctx, s.source.Id.Hex(), *s.clinician.UserId)
		if err != nil && !errors.Is(err, clinicians.ErrNotFound) {
			return plan, err
		}
		if sourceClinician != nil {
			plan.ClinicianAction = ClinicianActionMergeInto
			plan.Workspaces = append(plan.Workspaces, *s.source.Name)
			sort.Strings(plan.Workspaces)
		}
	}

	return plan, nil
}

type ClinicianPlanExecutor struct {
	logger               *zap.SugaredLogger
	cliniciansCollection *mongo.Collection
}

func NewClinicianPlanExecutor(logger *zap.SugaredLogger, db *mongo.Database) *ClinicianPlanExecutor {
	return &ClinicianPlanExecutor{
		logger:               logger,
		cliniciansCollection: db.Collection(clinicians.CollectionName),
	}
}

func (c *ClinicianPlanExecutor) Execute(ctx context.Context, plan ClinicianPlan, target clinics.Clinic) error {
	var id, idType string
	if plan.Clinician.UserId != nil {
		id = *plan.Clinician.UserId
		idType = "userId"
	} else {
		id = plan.Clinician.Id.Hex()
		idType = "id"
	}

	switch plan.ClinicianAction {
	case ClinicianActionMove:

		c.logger.Infow(
			"moving clinician",
			"clinicId", plan.Clinician.ClinicId.Hex(),
			idType, id,
			"targetClinicId", target.Id.Hex(),
		)
		return c.moveClinician(ctx, plan, target)
	case ClinicianActionMerge:
		// We don't need to merge clinician attributes, because we keep the roles of the target
		c.logger.Infow(
			"removing clinician",
			"clinicId", plan.Clinician.ClinicId.Hex(),
			idType, id,
		)
		return c.removeClinician(ctx, plan)
	case ClinicianActionMergeInto, ClinicianActionRetain:
		// We don't need to merge clinician attributes, just log a messages
		c.logger.Infow(
			"skipping clinician plan - nothing to do",
			"clinicId", plan.Clinician.ClinicId.Hex(),
			idType, id,
			"action", plan.ClinicianAction,
		)
		return nil
	default:
		return fmt.Errorf("unexpected plan action %s", plan.ClinicianAction)
	}
}

func (c *ClinicianPlanExecutor) moveClinician(ctx context.Context, plan ClinicianPlan, target clinics.Clinic) error {
	selector := bson.M{
		"_id": *plan.Clinician.Id,
	}

	update := bson.M{
		"$set": bson.M{
			"clinicId":    target.Id,
			"updatedTime": time.Now(),
		},
	}

	res, err := c.cliniciansCollection.UpdateOne(ctx, selector, update)
	if err != nil {
		return fmt.Errorf("error moving clinician: %w", err)
	}
	if res.ModifiedCount != 1 {
		return fmt.Errorf("error moving clinician: unexpected modified count %v", res.ModifiedCount)
	}
	return nil
}

func (c *ClinicianPlanExecutor) removeClinician(ctx context.Context, plan ClinicianPlan) error {
	selector := bson.M{
		"_id": *plan.Clinician.Id,
	}

	res, err := c.cliniciansCollection.DeleteOne(ctx, selector)
	if err != nil {
		return fmt.Errorf("error removing clinician: %w", err)
	}
	if res.DeletedCount != 1 {
		return fmt.Errorf("error removing clinician: unexpected modified count %v", res.DeletedCount)
	}
	return nil
}
