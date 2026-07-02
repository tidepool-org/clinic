package command

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"

	redoxPostgres "github.com/tidepool-org/clinic/redox/postgres"
	storepg "github.com/tidepool-org/clinic/store/postgres"
)

type pruneParams struct {
	fx.In

	Client *storepg.Client
	Logger *zap.SugaredLogger
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete expired rows from tables with retention policies",
	Long: "Replaces the Mongo TTL indexes: deletes mirrored scheduled summary " +
		"and reports orders older than the retention period. Intended to run " +
		"periodically, e.g. as a Kubernetes CronJob.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return Run(func(params pruneParams) error {
			return runPrune(cmd.Context(), params)
		})
	},
}

func runPrune(ctx context.Context, params pruneParams) error {
	if !params.Client.Enabled() {
		return fmt.Errorf("postgres is not enabled, set TIDEPOOL_POSTGRES_ENABLED=true")
	}

	writer := redoxPostgres.NewWriter(params.Client, params.Logger)
	pruned, err := writer.PruneScheduledOrders(ctx)
	if err != nil {
		return fmt.Errorf("error pruning scheduled summary and reports orders: %w", err)
	}
	fmt.Printf("scheduled_summary_reports_orders: pruned %d rows\n", pruned)
	return nil
}

func init() {
	rootCmd.AddCommand(pruneCmd)
}
