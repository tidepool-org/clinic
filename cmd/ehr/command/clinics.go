package command

import (
	"github.com/spf13/cobra"
)

var clinicsCmd = &cobra.Command{
	Use:   "clinics",
	Short: "EHR Clinics",
	Long:  "The clinics command is used to manage EHR clinics",
}

func init() {
	rootCmd.AddCommand(clinicsCmd)
}
