package merge

import "context"

type Task[T any] interface {
	Runnable
	GetResult() (TaskResult[T], error)
}

type TaskResult[T any] struct {
	ReportDetails T    `bson:"reportDetails"`
	PreventsMerge bool `bson:"preventsMerge"`
}

type Runnable interface {
	CanRun() bool
	DryRun(ctx context.Context) error
	Run(ctx context.Context) error
}

type GetRunner func(r Runnable) func(ctx context.Context) error

func Runner(r Runnable) func(ctx context.Context) error {
	return r.Run
}

func DryRunner(r Runnable) func(ctx context.Context) error {
	return r.DryRun
}
