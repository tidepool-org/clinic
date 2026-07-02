# PostgreSQL migration runbook

The clinic service mirrors every MongoDB write to PostgreSQL ("dual
writes") while MongoDB remains the source of truth. Reads stay on MongoDB.
Mirror writes are best-effort: a PostgreSQL failure is logged and counted
but never fails a request, and inside MongoDB transactions mirror writes
are buffered and flushed only after the transaction commits.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `TIDEPOOL_POSTGRES_ENABLED` | `false` | Master switch. When off the service never connects to PostgreSQL. |
| `TIDEPOOL_POSTGRES_HOST` / `_PORT` | `localhost` / `5432` | Server address. |
| `TIDEPOOL_POSTGRES_USERNAME` / `_PASSWORD` | `postgres` / empty | Credentials. |
| `TIDEPOOL_POSTGRES_DATABASE_NAME` | `clinic` | Database name. |
| `TIDEPOOL_POSTGRES_SSL_MODE` | `disable` | `sslmode` connection parameter. |
| `TIDEPOOL_POSTGRES_RUN_MIGRATIONS` | `true` | Apply schema migrations on startup (advisory-locked, replica safe). |
| `TIDEPOOL_POSTGRES_CONNECT_TIMEOUT_SECONDS` | `10` | Connection timeout; bounds how long a mirror write can stall on an outage (mirror operations also carry a 5s overall timeout). |

With `TIDEPOOL_POSTGRES_ENABLED=true` the service pings PostgreSQL and runs
migrations at startup — a down PostgreSQL fails **deployment**, never
requests: a runtime outage only produces error logs and metrics while the
API keeps serving from MongoDB.

## Rollout order

1. Deploy the release with `TIDEPOOL_POSTGRES_ENABLED=false` (no behavior
   change).
2. Provision the database and run `pgsync migrate` (or rely on the startup
   migration with the flag enabled in the next step).
3. Enable `TIDEPOOL_POSTGRES_ENABLED=true`. From this point every write is
   mirrored.
4. Backfill existing data per collection, parents before children of the
   read model (order below). Backfill is resumable and safe to run while
   dual writes are live — both paths use the same idempotent snapshot
   upserts:

   ```sh
   pgsync backfill --collection clinics
   pgsync backfill --collection clinicians
   pgsync backfill --collection patients
   pgsync backfill   # everything else (deletions, migrations, merge plans, xealth, redox)
   ```

5. Verify: `pgsync verify` (exits non-zero on drift). Re-run with
   `--repair` to converge.
6. Schedule recurring jobs (Kubernetes CronJobs):
   - `pgsync prune` daily — replaces the MongoDB TTL index by deleting
     `scheduled_summary_reports_orders` older than 90 days.
   - `pgsync verify --repair --sample 1000` daily — converges the known
     backfill-only write paths (below) and any operational drift.

## pgsync commands

| Command | Purpose |
|---|---|
| `pgsync migrate` | Apply schema migrations. |
| `pgsync backfill [--collection a,b] [--batch-size N] [--restart]` | Copy MongoDB collections into PostgreSQL in resumable id-ordered batches. |
| `pgsync verify [--collection a,b] [--repair] [--sample N] [--batch-size N]` | Reconcile stores: counts, sorted identity diff (missing/phantom rows), and with `--repair` convergence through the idempotent upserts plus deletion of phantom rows. `--sample N` re-upserts N random matched documents per collection to converge content drift the identity diff cannot see. |
| `pgsync prune` | Delete scheduled summary/report orders older than 90 days. |

All commands need the service environment (MongoDB and
`TIDEPOOL_POSTGRES_*` variables).

## Known convergence gaps (backfill-only paths)

Two write paths cannot be mirrored synchronously and converge through
backfill/verify instead:

1. **Scheduled summary and report orders** — both reschedule entry points
   write through a MongoDB `$merge` aggregation pipeline that executes
   server-side (see `patients/repository`). New and updated orders appear
   in PostgreSQL on the next `backfill`/`verify --repair` run.
2. **Clinic merges** — the merge executors mutate clinics and patients
   through raw collection handles (admins `$addToSet`, patient moves).
   Run `pgsync verify --repair` (or targeted backfills of `clinics`,
   `clinicians` and `patients`) after executing a clinic merge.

The `migrations` collection is reconciled by user id (its PostgreSQL
identity) rather than by document id.

## Monitoring

Prometheus metrics are exposed on `/metrics`:

- `clinic_dualwrite_total{entity,operation,outcome}` — alert on a non-zero
  rate of `outcome="error"` (or `"panic"`); sustained errors mean MongoDB
  and PostgreSQL are drifting and a `verify --repair` is needed once the
  cause is fixed.
- `clinic_dualwrite_duration_seconds` — mirror write latency.

## Phase 3 (read cutover)

Reads currently stay on MongoDB. Every field used for filtering, sorting
or aggregation is a real PostgreSQL column or child table, so reads can be
cut over per query path. Read-tuning indexes (the meeting-targets partial
indexes, sort composites) are deliberately deferred to the cutover to keep
dual-write amplification low.
