package store

import (
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewClient(host string) (*mongo.Client, error) {
	ctx := NewDbContext()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(host))
	if err != nil {
		return nil, fmt.Errorf("unable to connect to mongo: %w", err)
	}

	return client, nil
}
