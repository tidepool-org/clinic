package store

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

type Transaction = func(sessCtx mongo.SessionContext) (interface{}, error)

func WithTransaction(ctx context.Context, dbClient *mongo.Client, txn Transaction) (interface{}, error){
	session, err := dbClient.StartSession()
	if err != nil {
		return nil, fmt.Errorf("unable to start sessions %w", err)
	}
	defer session.EndSession(ctx)

	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)
	return session.WithTransaction(ctx, txn, txnOpts)
}
