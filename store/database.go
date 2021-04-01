package store

import (
	"go.mongodb.org/mongo-driver/mongo"
)

func NewDatabase(client *mongo.Client) (*mongo.Database, error){
	return client.Database(DefaultClinicDatabaseName), nil
}
