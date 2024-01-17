package store

import (
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewClient(config *Config) (*mongo.Client, error) {
	connectionString, err := config.GetConnectionString()
	if err != nil {
		return nil, err
	}
	bsonOpts := &options.BSONOptions{
		UseJSONStructTags: true,
	}
	opts := options.Client().ApplyURI(connectionString).SetBSONOptions(bsonOpts)

	ctx := NewDbContext()
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to mongo: %w", err)
	}

	return client, nil
}
