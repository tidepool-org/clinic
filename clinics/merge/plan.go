package merge

import (
	"context"
)

type Plan interface {
	PreventsMerge() bool
}

type Planner[T Plan] interface {
	Plan(ctx context.Context) (T, error)
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
		if s.PreventsMerge() == true {
			return true
		}
	}
	return false
}
