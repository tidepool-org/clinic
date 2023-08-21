package store

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewClient(host string) (*mongo.Client, error) {
	monitor := &event.CommandMonitor{
		Started: func(_ context.Context, e *event.CommandStartedEvent) {
			fmt.Println(e.Command)
		},
		Succeeded: func(_ context.Context, e *event.CommandSucceededEvent) {
			fmt.Println(e.Reply)
		},
	}

	ctx := NewDbContext()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(host).SetMonitor(monitor))
	if err != nil {
		return nil, fmt.Errorf("unable to connect to mongo: %w", err)
	}

	return client, nil
}
