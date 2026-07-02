package command

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tidepool-org/clinic/store/postgres"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply all pending PostgreSQL schema migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := postgres.NewConfig()
		if err != nil {
			return err
		}
		if err := postgres.Migrate(context.Background(), config); err != nil {
			return err
		}
		fmt.Println("migrations applied successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
