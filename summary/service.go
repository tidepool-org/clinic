package summary

import (
	"context"
	"github.com/tidepool-org/clinic/store"
	"go.uber.org/zap"
)

type service[T Period] struct {
	repo   Repository[T]
	logger *zap.SugaredLogger
}

var _ Service = &service[CGMPeriod]{}
var _ Service = &service[BGMPeriod]{}

func NewService[T Period](repo Repository[T], logger *zap.SugaredLogger) (Service[T], error) {
	return &service{
		repo:   repo,
		logger: logger,
	}, nil
}

func (s *service[T]) Get(ctx context.Context, userId string) (*Summary[T], error) {
	return s.repo.Get(ctx, userId)
}

func (s *service[T]) List(ctx context.Context, filter *Filter, pagination store.Pagination, sorts []*store.Sort) (*ListResult[T], error) {
	return s.repo.List(ctx, filter, pagination, sorts)
}

func (s *service[T]) Remove(ctx context.Context, userId string) error {
	return s.repo.Remove(ctx, userId)
}

func (s *service[T]) CreateOrUpdate(ctx context.Context, summary *Summary[T]) error {
	return s.repo.CreateOrUpdate(ctx, summary)
}
