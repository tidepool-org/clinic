package store

import (
	// Built-in Golang packages
	"context" // manage multiple requests
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

var (
	ContextTimeout = time.Duration(20) * time.Second
)

type MongoPagingParams struct {
	Offset int64
	Limit  int64
}

type Pagination struct {
	Offset int
	Limit  int
}

func DefaultPagination() Pagination {
	return Pagination{
		Offset: 0,
		Limit:  10,
	}
}

type Sort struct {
	Attribute string
	Ascending bool
}

func (s *Sort) Order() int {
	if s.Ascending {
		return 1
	}
	return -1
}

func ObjectIDSFromStringArray(ids []string) []primitive.ObjectID {
	objectIds := make([]primitive.ObjectID, 0, len(ids))
	for _, id := range ids {
		if objectId, err := primitive.ObjectIDFromHex(id); err == nil {
			objectIds = append(objectIds, objectId)
		}
	}
	return objectIds
}

func NewDbContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), ContextTimeout)
	return ctx
}
