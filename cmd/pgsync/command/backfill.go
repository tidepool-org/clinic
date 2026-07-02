package command

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
	"go.uber.org/zap"

	cliniciansPostgres "github.com/tidepool-org/clinic/clinicians/postgres"
	mergePostgres "github.com/tidepool-org/clinic/clinics/merge/postgres"
	migrationPostgres "github.com/tidepool-org/clinic/clinics/migration/postgres"
	clinicsPostgres "github.com/tidepool-org/clinic/clinics/postgres"
	deletionsPostgres "github.com/tidepool-org/clinic/deletions/postgres"
	patientsPostgres "github.com/tidepool-org/clinic/patients/postgres"
	redoxPostgres "github.com/tidepool-org/clinic/redox/postgres"
	storepg "github.com/tidepool-org/clinic/store/postgres"
	"github.com/tidepool-org/clinic/store/postgres/sqlcgen"
	xealthPostgres "github.com/tidepool-org/clinic/xealth/postgres"
)

var (
	backfillCollections []string
	backfillBatchSize   int
	backfillRestart     bool
)

// backfillers provides every registered backfiller. New domains register
// their constructors here.
var backfillers = fx.Provide(
	asBackfiller(func(db *mongo.Database, writer *deletionsPostgres.Writer) storepg.Backfiller {
		return deletionsPostgres.NewBackfiller(deletionsPostgres.TypePatient, db, writer)
	}),
	asBackfiller(func(db *mongo.Database, writer *deletionsPostgres.Writer) storepg.Backfiller {
		return deletionsPostgres.NewBackfiller(deletionsPostgres.TypeClinician, db, writer)
	}),
	asBackfiller(func(db *mongo.Database, writer *deletionsPostgres.Writer) storepg.Backfiller {
		return deletionsPostgres.NewBackfiller(deletionsPostgres.TypeClinic, db, writer)
	}),
	asBackfiller(migrationPostgres.NewBackfiller),
	asBackfiller(mergePostgres.NewBackfiller),
	asBackfiller(xealthPostgres.NewPreorderBackfiller),
	asBackfiller(xealthPostgres.NewOrderBackfiller),
	asBackfiller(xealthPostgres.NewReportViewBackfiller),
	asBackfiller(redoxPostgres.NewMessageBackfiller),
	asBackfiller(redoxPostgres.NewScheduledOrderBackfiller),
	asBackfiller(clinicsPostgres.NewBackfiller),
	asBackfiller(cliniciansPostgres.NewBackfiller),
	asBackfiller(patientsPostgres.NewBackfiller),
	deletionsPostgres.NewWriter,
)

func asBackfiller(f any) any {
	return fx.Annotate(f, fx.ResultTags(`group:"backfillers"`))
}

type backfillParams struct {
	fx.In

	Backfillers []storepg.Backfiller `group:"backfillers"`
	Client      *storepg.Client
	Logger      *zap.SugaredLogger
}

var backfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Copy Mongo collections into their PostgreSQL tables",
	Long: "Copies documents in batches ordered by id using the same idempotent " +
		"upserts as the dual-write path. Progress is persisted per collection, " +
		"so an interrupted backfill resumes where it left off. Safe to run " +
		"while dual writes are enabled.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return Run(func(params backfillParams) error {
			return runBackfill(cmd.Context(), params)
		}, backfillers)
	},
}

func runBackfill(ctx context.Context, params backfillParams) error {
	if !params.Client.Enabled() {
		return fmt.Errorf("postgres is not enabled, set TIDEPOOL_POSTGRES_ENABLED=true")
	}
	queries := sqlcgen.New(params.Client.Pool())

	for _, backfiller := range params.Backfillers {
		collection := backfiller.Collection()
		if len(backfillCollections) > 0 && !slices.Contains(backfillCollections, collection) {
			continue
		}

		cursor := primitive.NilObjectID
		if backfillRestart {
			if err := queries.DeleteBackfillProgress(ctx, collection); err != nil {
				return fmt.Errorf("error resetting progress of %s: %w", collection, err)
			}
		} else {
			progress, err := queries.GetBackfillProgress(ctx, collection)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("error reading progress of %s: %w", collection, err)
			}
			if err == nil && progress.LastID != "" {
				cursor, err = primitive.ObjectIDFromHex(progress.LastID)
				if err != nil {
					return fmt.Errorf("invalid progress cursor %q of %s: %w", progress.LastID, collection, err)
				}
			}
		}

		total := 0
		for {
			last, n, err := backfiller.BackfillBatch(ctx, cursor, backfillBatchSize)
			if err != nil {
				return fmt.Errorf("error backfilling %s: %w", collection, err)
			}
			if n == 0 {
				break
			}
			if err := queries.UpsertBackfillProgress(ctx, sqlcgen.UpsertBackfillProgressParams{
				Collection: collection,
				LastID:     last.Hex(),
			}); err != nil {
				return fmt.Errorf("error persisting progress of %s: %w", collection, err)
			}
			cursor = last
			total += n
			params.Logger.Infow("backfill progress", "collection", collection, "count", total)
		}
		fmt.Printf("%s: backfilled %d documents\n", collection, total)
	}

	return nil
}

func init() {
	backfillCmd.Flags().StringSliceVar(&backfillCollections, "collection", nil, "Collections to backfill (default all)")
	backfillCmd.Flags().IntVar(&backfillBatchSize, "batch-size", 500, "Number of documents per batch")
	backfillCmd.Flags().BoolVar(&backfillRestart, "restart", false, "Ignore persisted progress and start from the beginning")
	rootCmd.AddCommand(backfillCmd)
}
