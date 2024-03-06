package store

import (
	"go.mongodb.org/mongo-driver/mongo"
)

func NewDatabase(client *mongo.Client, cfg *Config) (*mongo.Database, error) {
	return client.Database(cfg.DatabaseName), nil
}
