package store

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"

	"github.com/tidepool-org/clinic/store/dualwrite"
)

type Transaction = func(sessCtx mongo.SessionContext) (interface{}, error)

func WithTransaction(ctx context.Context, dbClient *mongo.Client, txn Transaction) (interface{}, error) {
	session, err := dbClient.StartSession()
	if err != nil {
		return nil, fmt.Errorf("unable to start sessions %w", err)
	}
	defer session.EndSession(ctx)

	// Postgres mirror operations executed during the transaction are
	// buffered on the context and flushed only after the transaction
	// commits, so aborted transactions never leave phantom rows behind.
	ctx, queue := dualwrite.WithQueue(ctx)

	txnOpts := options.
		Transaction().
		SetWriteConcern(writeconcern.Majority()).
		SetReadConcern(readconcern.Snapshot())
	result, err := session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// The driver retries the callback on transient errors; drop mirror
		// operations enqueued by a previous aborted attempt.
		queue.Reset()
		return txn(sessCtx)
	}, txnOpts)
	if err != nil {
		return result, err
	}

	queue.Flush(context.WithoutCancel(ctx))
	return result, nil
}
