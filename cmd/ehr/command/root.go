package command

import (
	"fmt"
	"github.com/DataDog/datadog-agent/pkg/util/fxutil"
	"github.com/spf13/cobra"
	"github.com/tidepool-org/clinic/api"
	"go.uber.org/fx"
	"os"
)

var logLevel string

// Run executes a given function with dependencies supplied by the clinic service DI graph
// `f` must return an error or nothing
// `opts` can be used to supply additional arguments that are not provided by the clinic service
func Run(f interface{}, opts ...fx.Option) error {
	deps := append(opts, api.Dependencies()...)
	return fxutil.OneShot(f, deps...)
}

var rootCmd = &cobra.Command{
	Use:   "ehr",
	Short: "Helper tool to to manage EHR integrations",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Overwrite zap's log level
		return os.Setenv("LOG_LEVEL", logLevel)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "v", "error", "Log Level")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
