package command

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
	"go.uber.org/zap"

	storepg "github.com/tidepool-org/clinic/store/postgres"
)

var (
	verifyCollections []string
	verifyBatchSize   int
	verifySample      int
	verifyRepair      bool
)

const verifyMaxReportedIds = 20

type verifyParams struct {
	fx.In

	Verifiers []storepg.Verifier `group:"backfillers"`
	Client    *storepg.Client
	Database  *mongo.Database
	Logger    *zap.SugaredLogger
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Reconcile Mongo collections with their PostgreSQL tables",
	Long: "Compares each collection with its PostgreSQL mirror in three tiers: " +
		"document counts, a sorted identity diff reporting missing and phantom " +
		"rows, and (with --repair) convergence through the same idempotent " +
		"upserts used by dual writes and backfill. --sample N additionally " +
		"re-upserts N random matched documents per collection to converge " +
		"content drift the identity comparison cannot see (applied with " +
		"--repair only). Exits non-zero when drift is found and not repaired.",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return Run(func(params verifyParams) error {
			return runVerify(cmd.Context(), params)
		}, backfillers)
	},
}

func runVerify(ctx context.Context, params verifyParams) error {
	if !params.Client.Enabled() {
		return fmt.Errorf("postgres is not enabled, set TIDEPOOL_POSTGRES_ENABLED=true")
	}
	collections := make([]storepg.Backfiller, 0, len(params.Verifiers))
	for _, verifier := range params.Verifiers {
		collections = append(collections, verifier)
	}
	if err := validateCollections(verifyCollections, collections); err != nil {
		return err
	}

	drifted := make([]string, 0)
	failed := make([]string, 0)
	for _, verifier := range params.Verifiers {
		collection := verifier.Collection()
		if len(verifyCollections) > 0 && !slices.Contains(verifyCollections, collection) {
			continue
		}

		report, err := storepg.VerifyCollection(ctx, params.Client.Pool(), params.Database, verifier, storepg.VerifyOptions{
			BatchSize: verifyBatchSize,
			Sample:    verifySample,
			Repair:    verifyRepair,
		})
		if err != nil {
			// A failure in one collection must not prevent the remaining
			// collections from being verified
			params.Logger.Errorw("error verifying collection", "collection", collection, "error", err)
			failed = append(failed, collection)
			continue
		}

		printReport(report)
		if report.HasDrift() && !report.Repaired {
			drifted = append(drifted, collection)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("verification failed for %s", strings.Join(failed, ", "))
	}
	if len(drifted) > 0 {
		return fmt.Errorf("drift detected in %s; run with --repair to converge", strings.Join(drifted, ", "))
	}
	return nil
}

// validateCollections rejects unknown --collection values so a typo doesn't
// silently verify or backfill nothing and exit 0.
func validateCollections(requested []string, available []storepg.Backfiller) error {
	if len(requested) == 0 {
		return nil
	}
	known := make([]string, 0, len(available))
	for _, backfiller := range available {
		known = append(known, backfiller.Collection())
	}
	for _, name := range requested {
		if !slices.Contains(known, name) {
			slices.Sort(known)
			return fmt.Errorf("unknown collection %q; valid collections: %s", name, strings.Join(known, ", "))
		}
	}
	return nil
}

func printReport(report *storepg.VerificationReport) {
	fmt.Printf("%s: mongo=%d pg=%d matched=%d missing=%d phantom=%d",
		report.Collection, report.MongoCount, report.PGCount, report.Matched,
		len(report.Missing), len(report.Phantom))
	if report.Repaired {
		fmt.Printf(" resynced=%d deleted=%d", report.Resynced, report.Deleted)
	}
	fmt.Println()

	printIds := func(label string, ids []string) {
		if len(ids) == 0 {
			return
		}
		shown := ids
		suffix := ""
		if len(shown) > verifyMaxReportedIds {
			shown = shown[:verifyMaxReportedIds]
			suffix = fmt.Sprintf(" ... (%d total)", len(ids))
		}
		fmt.Printf("  %s: %s%s\n", label, strings.Join(shown, ", "), suffix)
	}
	printIds("missing in postgres", report.Missing)
	printIds("phantom in postgres", report.Phantom)
}

func init() {
	verifyCmd.Flags().StringSliceVar(&verifyCollections, "collection", nil, "Collections to verify (default all)")
	verifyCmd.Flags().IntVar(&verifyBatchSize, "batch-size", 500, "Number of identities per comparison batch")
	verifyCmd.Flags().IntVar(&verifySample, "sample", 0, "Number of random matched documents to re-upsert per collection (with --repair)")
	verifyCmd.Flags().BoolVar(&verifyRepair, "repair", false, "Re-upsert missing and sampled documents and delete phantom rows")
	rootCmd.AddCommand(verifyCmd)
}
