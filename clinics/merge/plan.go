package merge

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Plan interface {
	PreventsMerge() bool
	Errors() []ReportError
}

type Planner[T Plan] interface {
	Plan(ctx context.Context) (T, error)
}

type PersistentPlan[T Plan] struct {
	Id     *primitive.ObjectID `bson:"_id,omitempty"`
	Plan   T                   `bson:"plan"`
	PlanId primitive.ObjectID  `bson:"planId"`
	Type   string              `bson:"type"`
}

func NewPersistentPlan[T Plan](planId primitive.ObjectID, typ string, p T) PersistentPlan[T] {
	// The id is generated client-side so the document has the same identity
	// in every store it is persisted to
	id := primitive.NewObjectID()
	return PersistentPlan[T]{
		Id:     &id,
		Plan:   p,
		PlanId: planId,
		Type:   typ,
	}
}

func (p PersistentPlan[T]) PlanDocumentId() string {
	if p.Id == nil {
		return ""
	}
	return p.Id.Hex()
}

func (p PersistentPlan[T]) PlanGroupId() string {
	return p.PlanId.Hex()
}

func (p PersistentPlan[T]) PlanType() string {
	return p.Type
}

func (p PersistentPlan[T]) PlanPayload() any {
	return p.Plan
}

func RunPlanners[T Plan](ctx context.Context, planners []Planner[T]) ([]T, error) {
	result := make([]T, 0, len(planners))
	for _, planner := range planners {
		res, err := planner.Plan(ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, res)
	}
	return result, nil
}

func PlansPreventMerge[T Plan](plans []T) bool {
	for _, s := range plans {
		if s.PreventsMerge() {
			return true
		}
	}
	return false
}

func PlansErrors[T Plan](plans []T) []ReportError {
	errs := make([]ReportError, 0)
	for _, s := range plans {
		errs = append(errs, s.Errors()...)
	}
	return errs
}
