// Package dualwrite provides the best-effort mirroring primitive used to
// replicate MongoDB writes to PostgreSQL while MongoDB remains the source of
// truth. Mirror operations never fail the request: errors and panics are
// logged and counted, then dropped. Inside a MongoDB transaction, mirror
// operations are queued on the context and flushed only after the
// transaction commits, so PostgreSQL never observes state from aborted
// transactions.
package dualwrite

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

const executeTimeout = 5 * time.Second

var (
	opsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "clinic_dualwrite_total",
		Help: "Count of postgres dual-write mirror operations by outcome.",
	}, []string{"entity", "operation", "outcome"})

	opsDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "clinic_dualwrite_duration_seconds",
		Help:    "Duration of postgres dual-write mirror operations.",
		Buckets: prometheus.DefBuckets,
	}, []string{"entity", "operation"})
)

type queueKey struct{}

// Queue buffers mirror operations enqueued during a MongoDB transaction so
// they can be flushed after the transaction commits.
type Queue struct {
	mu  sync.Mutex
	ops []func(context.Context)
}

// WithQueue attaches a fresh queue to the context and returns both. Mirror
// operations executed with the returned context are buffered until Flush.
func WithQueue(ctx context.Context) (context.Context, *Queue) {
	queue := &Queue{}
	return context.WithValue(ctx, queueKey{}, queue), queue
}

func queueFromContext(ctx context.Context) *Queue {
	queue, _ := ctx.Value(queueKey{}).(*Queue)
	return queue
}

func (q *Queue) add(op func(context.Context)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ops = append(q.ops, op)
}

// Reset drops all buffered operations. The MongoDB driver retries the
// transaction callback on transient errors; resetting at the top of each
// attempt prevents mirror operations from aborted attempts from being
// flushed.
func (q *Queue) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ops = nil
}

// Flush runs all buffered operations and empties the queue.
func (q *Queue) Flush(ctx context.Context) {
	q.mu.Lock()
	ops := q.ops
	q.ops = nil
	q.mu.Unlock()

	for _, op := range ops {
		op(ctx)
	}
}

// Execute runs a best-effort mirror operation. If the context carries a
// Queue (i.e. a MongoDB transaction is in progress) the operation is
// buffered until the transaction commits; otherwise it runs immediately.
// Failures are logged and counted but never returned, and the operation is
// detached from the caller's cancellation so an in-flight mirror write
// completes even when the request has already returned.
func Execute(ctx context.Context, logger *zap.SugaredLogger, entity, operation string, fn func(context.Context) error) {
	wrapped := func(runCtx context.Context) {
		start := time.Now()
		outcome := "success"
		defer func() {
			if r := recover(); r != nil {
				outcome = "panic"
				logger.Errorw("dual write panicked",
					"entity", entity, "operation", operation, "panic", r)
			}
			opsTotal.WithLabelValues(entity, operation, outcome).Inc()
			opsDuration.WithLabelValues(entity, operation).Observe(time.Since(start).Seconds())
		}()

		runCtx, cancel := context.WithTimeout(runCtx, executeTimeout)
		defer cancel()

		if err := fn(runCtx); err != nil {
			outcome = "error"
			logger.Errorw("dual write failed",
				"entity", entity, "operation", operation, "error", err)
		}
	}

	if queue := queueFromContext(ctx); queue != nil {
		queue.add(wrapped)
		return
	}
	wrapped(context.WithoutCancel(ctx))
}
