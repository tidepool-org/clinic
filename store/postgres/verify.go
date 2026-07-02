package postgres

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Verifier reconciles a Mongo collection with its Postgres tables. Every
// backfiller implements it; the identity used for reconciliation is the
// value of IDColumn in Postgres and MongoIDs on the Mongo side, compared as
// strings in byte order.
type Verifier interface {
	Backfiller
	// Table is the Postgres parent table mirroring the collection.
	Table() string
	// IDColumn is the Postgres column holding the identity shared with Mongo.
	IDColumn() string
	// MongoIDs returns up to limit identities greater than after in byte order.
	MongoIDs(ctx context.Context, after string, limit int) ([]string, error)
	// ResyncBatch re-runs the idempotent upserts for the documents with the
	// given identities, converging any content drift.
	ResyncBatch(ctx context.Context, ids []string) error
}

// CollectionSync is the standard Verifier for collections whose Postgres
// identity is the hex encoded Mongo object id. Domains construct one with
// their collection, parent table and upsert callback; migrations-style
// collections with a different identity embed it and override the identity
// methods.
type CollectionSync struct {
	collection *mongo.Collection
	table      string
	upsert     func(ctx context.Context, docs []bson.M) error
}

func NewCollectionSync(collection *mongo.Collection, table string, upsert func(ctx context.Context, docs []bson.M) error) *CollectionSync {
	return &CollectionSync{
		collection: collection,
		table:      table,
		upsert:     upsert,
	}
}

func (s *CollectionSync) Collection() string {
	return s.collection.Name()
}

func (s *CollectionSync) Table() string {
	return s.table
}

func (s *CollectionSync) IDColumn() string {
	return "id"
}

func (s *CollectionSync) BackfillBatch(ctx context.Context, after primitive.ObjectID, limit int) (primitive.ObjectID, int, error) {
	return BackfillDocuments(ctx, s.collection, after, limit, s.upsert)
}

func (s *CollectionSync) MongoIDs(ctx context.Context, after string, limit int) ([]string, error) {
	selector := bson.M{}
	if after != "" {
		id, err := primitive.ObjectIDFromHex(after)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor %q for %s: %w", after, s.Collection(), err)
		}
		selector["_id"] = bson.M{"$gt": id}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).
		SetLimit(int64(limit)).
		SetProjection(bson.M{"_id": 1})
	cursor, err := s.collection.Find(ctx, selector, opts)
	if err != nil {
		return nil, fmt.Errorf("error listing ids in %s: %w", s.Collection(), err)
	}

	var docs []struct {
		Id primitive.ObjectID `bson:"_id"`
	}
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("error decoding ids in %s: %w", s.Collection(), err)
	}

	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.Id.Hex())
	}
	return ids, nil
}

func (s *CollectionSync) ResyncBatch(ctx context.Context, ids []string) error {
	objectIds := make([]primitive.ObjectID, 0, len(ids))
	for _, id := range ids {
		objectId, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return fmt.Errorf("invalid id %q for %s: %w", id, s.Collection(), err)
		}
		objectIds = append(objectIds, objectId)
	}

	cursor, err := s.collection.Find(ctx, bson.M{"_id": bson.M{"$in": objectIds}})
	if err != nil {
		return fmt.Errorf("error listing documents in %s: %w", s.Collection(), err)
	}
	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return fmt.Errorf("error decoding documents in %s: %w", s.Collection(), err)
	}
	if len(docs) == 0 {
		return nil
	}
	return s.upsert(ctx, docs)
}

// VerificationReport is the result of reconciling one collection.
type VerificationReport struct {
	Collection string
	MongoCount int64
	PGCount    int64
	// Missing are Mongo identities without a Postgres row.
	Missing []string
	// Phantom are Postgres identities without a Mongo document.
	Phantom []string
	Matched int64
	// Resynced counts documents re-upserted by --repair: all missing ones
	// plus the sampled matched ones.
	Resynced int
	// Deleted counts phantom rows removed by --repair.
	Deleted int
	// Repaired reports whether repairs were applied.
	Repaired bool
}

func (r *VerificationReport) HasDrift() bool {
	return r.MongoCount != r.PGCount || len(r.Missing) > 0 || len(r.Phantom) > 0
}

// VerifyOptions controls reconciliation.
type VerifyOptions struct {
	// BatchSize is the id page size and the resync chunk size.
	BatchSize int
	// Sample is the number of matched documents to re-upsert with Repair to
	// converge content drift the id comparison cannot see.
	Sample int
	// Repair applies fixes: missing and sampled documents are re-upserted,
	// phantom rows are deleted.
	Repair bool
}

// VerifyCollection reconciles a collection in three tiers: counts, a sorted
// identity merge-diff, and (with Repair) convergence through the idempotent
// upserts.
func VerifyCollection(ctx context.Context, pool *pgxpool.Pool, db *mongo.Database, verifier Verifier, opts VerifyOptions) (*VerificationReport, error) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 500
	}
	report := &VerificationReport{Collection: verifier.Collection()}

	mongoCount, err := db.Collection(verifier.Collection()).CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("error counting documents in %s: %w", verifier.Collection(), err)
	}
	report.MongoCount = mongoCount

	// The table and column names are trusted constants provided by the
	// domain packages, not user input.
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, verifier.Table())).Scan(&report.PGCount); err != nil {
		return nil, fmt.Errorf("error counting rows in %s: %w", verifier.Table(), err)
	}

	sample := newReservoir(opts.Sample)
	if err := mergeDiff(ctx, pool, verifier, opts.BatchSize, report, sample); err != nil {
		return nil, err
	}

	if opts.Repair {
		report.Repaired = true
		for chunk := range chunked(report.Missing, opts.BatchSize) {
			if err := verifier.ResyncBatch(ctx, chunk); err != nil {
				return nil, fmt.Errorf("error resyncing %s: %w", verifier.Collection(), err)
			}
			report.Resynced += len(chunk)
		}
		for chunk := range chunked(sample.items, opts.BatchSize) {
			if err := verifier.ResyncBatch(ctx, chunk); err != nil {
				return nil, fmt.Errorf("error resyncing sampled documents of %s: %w", verifier.Collection(), err)
			}
			report.Resynced += len(chunk)
		}
		for chunk := range chunked(report.Phantom, opts.BatchSize) {
			tag, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE %s = ANY($1)`, verifier.Table(), verifier.IDColumn()), chunk)
			if err != nil {
				return nil, fmt.Errorf("error deleting phantom rows of %s: %w", verifier.Table(), err)
			}
			report.Deleted += int(tag.RowsAffected())
		}
	}

	return report, nil
}

// mergeDiff streams both identity sets in byte order and classifies every
// identity as missing, phantom or matched.
func mergeDiff(ctx context.Context, pool *pgxpool.Pool, verifier Verifier, batchSize int, report *VerificationReport, sample *reservoir) error {
	mongoIds := &idStream{fetch: func(after string) ([]string, error) {
		return verifier.MongoIDs(ctx, after, batchSize)
	}}
	// COLLATE "C" compares by bytes, matching both Mongo's object id
	// ordering (hex preserves byte order) and Go string comparison.
	pgIds := &idStream{fetch: func(after string) ([]string, error) {
		rows, err := pool.Query(ctx, fmt.Sprintf(
			`SELECT %[1]s FROM %[2]s WHERE %[1]s COLLATE "C" > $1 ORDER BY %[1]s COLLATE "C" LIMIT $2`,
			verifier.IDColumn(), verifier.Table()), after, batchSize)
		if err != nil {
			return nil, fmt.Errorf("error listing ids in %s: %w", verifier.Table(), err)
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}}

	m, mOk, err := mongoIds.next()
	if err != nil {
		return err
	}
	p, pOk, err := pgIds.next()
	if err != nil {
		return err
	}

	for mOk || pOk {
		switch {
		case mOk && (!pOk || m < p):
			report.Missing = append(report.Missing, m)
			if m, mOk, err = mongoIds.next(); err != nil {
				return err
			}
		case pOk && (!mOk || p < m):
			report.Phantom = append(report.Phantom, p)
			if p, pOk, err = pgIds.next(); err != nil {
				return err
			}
		default:
			report.Matched++
			sample.observe(m)
			if m, mOk, err = mongoIds.next(); err != nil {
				return err
			}
			if p, pOk, err = pgIds.next(); err != nil {
				return err
			}
		}
	}
	return nil
}

// idStream pulls pages of ascending identities through a keyset cursor.
type idStream struct {
	fetch func(after string) ([]string, error)
	buf   []string
	pos   int
	last  string
	done  bool
}

func (s *idStream) next() (string, bool, error) {
	if s.pos >= len(s.buf) && !s.done {
		buf, err := s.fetch(s.last)
		if err != nil {
			return "", false, err
		}
		s.buf, s.pos = buf, 0
		if len(buf) == 0 {
			s.done = true
		} else {
			s.last = buf[len(buf)-1]
		}
	}
	if s.pos >= len(s.buf) {
		return "", false, nil
	}
	id := s.buf[s.pos]
	s.pos++
	return id, true, nil
}

// reservoir keeps a uniform sample of the observed identities.
type reservoir struct {
	size  int
	seen  int
	items []string
}

func newReservoir(size int) *reservoir {
	return &reservoir{size: size}
}

func (r *reservoir) observe(id string) {
	if r.size <= 0 {
		return
	}
	r.seen++
	if len(r.items) < r.size {
		r.items = append(r.items, id)
		return
	}
	if n := rand.Intn(r.seen); n < r.size {
		r.items[n] = id
	}
}

func chunked(items []string, size int) func(func([]string) bool) {
	return func(yield func([]string) bool) {
		for start := 0; start < len(items); start += size {
			end := min(start+size, len(items))
			if !yield(items[start:end]) {
				return
			}
		}
	}
}
