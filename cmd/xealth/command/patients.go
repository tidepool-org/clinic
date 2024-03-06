package command

import (
	"github.com/spf13/cobra"
)

var patientsCmd = &cobra.Command{
	Use:   "patients",
	Short: "Xealth Patients",
	Long:  "The patients command is used to manage Xealth patients",
}
