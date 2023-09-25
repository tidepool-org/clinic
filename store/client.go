package store

import (
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewClient(host string) (*mongo.Client, error) {
	bsonOpts := &options.BSONOptions{
		UseJSONStructTags: true,
	}
	opts := options.Client().ApplyURI(host).SetBSONOptions(bsonOpts)

	ctx := NewDbContext()
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to mongo: %w", err)
	}

	return client, nil
}
