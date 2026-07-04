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

const (
	executeTimeout = 5 * time.Second
	// flushTimeout bounds an entire queue flush. Flushes run synchronously
	// after a Mongo transaction commits but before the request returns, and
	// a transaction may enqueue an unbounded number of mirror operations
	// (e.g. one per merge plan); without an overall budget an unreachable
	// Postgres would stall the request for len(queue) * executeTimeout.
	// Once the budget is exhausted the remaining operations fail immediately
	// on their expired context and are counted as errors, and backfill or
	// verify --repair converges the skipped writes.
	flushTimeout = 30 * time.Second
)

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

const (
	breakerThreshold = 5
	breakerCooldown  = 10 * time.Second
)

// breaker short-circuits mirror operations while Postgres is unhealthy so an
// outage costs requests nothing instead of executeTimeout per operation.
// After breakerThreshold consecutive failures it opens for breakerCooldown;
// operations arriving while open are dropped and counted with outcome
// "skipped". After the cooldown a single trial operation is let through
// (half-open): success closes the breaker, failure re-opens it. Skipped
// writes converge through pgsync backfill or verify --repair.
type breaker struct {
	mu        sync.Mutex
	failures  int
	openUntil time.Time
	halfOpen  bool
}

var sharedBreaker breaker

func (b *breaker) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failures < breakerThreshold {
		return true
	}
	if now.Before(b.openUntil) {
		return false
	}
	if b.halfOpen {
		// A trial operation is already in flight
		return false
	}
	b.halfOpen = true
	return true
}

func (b *breaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.halfOpen = false
	b.openUntil = time.Time{}
}

func (b *breaker) recordFailure(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	b.halfOpen = false
	if b.failures >= breakerThreshold {
		b.openUntil = now.Add(breakerCooldown)
	}
}

// ResetBreakerForTesting restores the shared circuit breaker to its closed
// state. Specs that intentionally fail mirror operations use it to avoid
// leaking breaker state into other specs.
func ResetBreakerForTesting() {
	sharedBreaker.mu.Lock()
	defer sharedBreaker.mu.Unlock()
	sharedBreaker.failures = 0
	sharedBreaker.halfOpen = false
	sharedBreaker.openUntil = time.Time{}
}

// ExpireBreakerCooldownForTesting moves an open breaker straight to the end
// of its cooldown so specs can exercise the half-open transition without
// sleeping.
func ExpireBreakerCooldownForTesting() {
	sharedBreaker.mu.Lock()
	defer sharedBreaker.mu.Unlock()
	sharedBreaker.openUntil = time.Time{}
}

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

// Flush runs all buffered operations and empties the queue. The whole flush
// shares a single time budget so a slow or unreachable Postgres cannot stall
// the calling request longer than flushTimeout.
func (q *Queue) Flush(ctx context.Context) {
	q.mu.Lock()
	ops := q.ops
	q.ops = nil
	q.mu.Unlock()

	if len(ops) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, flushTimeout)
	defer cancel()

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
		if !sharedBreaker.allow(time.Now()) {
			opsTotal.WithLabelValues(entity, operation, "skipped").Inc()
			logger.Debugw("dual write skipped, circuit open",
				"entity", entity, "operation", operation)
			return
		}

		start := time.Now()
		outcome := "success"
		defer func() {
			if r := recover(); r != nil {
				outcome = "panic"
				logger.Errorw("dual write panicked",
					"entity", entity, "operation", operation, "panic", r)
			}
			if outcome == "success" {
				sharedBreaker.recordSuccess()
			} else {
				sharedBreaker.recordFailure(time.Now())
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
