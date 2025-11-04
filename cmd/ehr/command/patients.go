package command

import (
	"github.com/spf13/cobra"
)

var patientsCmd = &cobra.Command{
	Use:   "patients",
	Short: "Manage EHR patients",
	Long:  "The patients command is used to manage patients in EHR enabled clinics",
}

func init() {
	rootCmd.AddCommand(patientsCmd)
}
