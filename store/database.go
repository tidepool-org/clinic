package store

import (
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewDatabase(host string) (*mongo.Database, error){
	fmt.Println("Creating Mongo Store")
	client, err := mongo.NewClient(options.Client().ApplyURI(host))
	if err != nil {
		return nil, fmt.Errorf("unable to create mongo client: %w", err)
	}

	ctx := NewDbContext()
	err = client.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to mongo: %w", err)
	}
	fmt.Println("Created Mongo Store Successfully")

	return client.Database(DefaultClinicDatabaseName), nil
}
