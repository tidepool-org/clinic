package postgres

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Backfiller copies one Mongo collection into its Postgres tables in batches
// using the same idempotent upserts as the dual-write path, so backfill and
// dual writes converge.
type Backfiller interface {
	Collection() string
	// BackfillBatch copies up to limit documents with ids greater than after
	// and returns the last copied id and the number of copied documents.
	BackfillBatch(ctx context.Context, after primitive.ObjectID, limit int) (primitive.ObjectID, int, error)
}

// BackfillDocuments reads a batch of raw documents ordered by id and hands
// them to upsert. It implements the shared portion of most Backfillers.
func BackfillDocuments(ctx context.Context, collection *mongo.Collection, after primitive.ObjectID, limit int, upsert func(ctx context.Context, docs []bson.M) error) (primitive.ObjectID, int, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).
		SetLimit(int64(limit))
	cursor, err := collection.Find(ctx, bson.M{"_id": bson.M{"$gt": after}}, opts)
	if err != nil {
		return after, 0, fmt.Errorf("error listing documents in %s: %w", collection.Name(), err)
	}

	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return after, 0, fmt.Errorf("error decoding documents in %s: %w", collection.Name(), err)
	}
	if len(docs) == 0 {
		return after, 0, nil
	}

	if err := upsert(ctx, docs); err != nil {
		return after, 0, err
	}

	last, ok := docs[len(docs)-1]["_id"].(primitive.ObjectID)
	if !ok {
		return after, 0, fmt.Errorf("document in %s has a non object id key", collection.Name())
	}
	return last, len(docs), nil
}
