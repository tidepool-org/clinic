package command

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/store"
)

var clinicsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List EHR Enabled Clinics",
	Long:  "The list command is used to retrieve a list of all EHR enabled clinics",
	RunE:  func(cmd *cobra.Command, args []string) error { return Run(listClinics) },
}

func listClinics(service clinics.Service) error {
	enabled := true
	page := store.DefaultPagination().WithLimit(1000)
	list, err := service.List(context.TODO(), &clinics.Filter{
		EHREnabled: &enabled,
	}, page)
	if err != nil {
		return err
	}

	for _, clinic := range list {
		id := clinic.Id.Hex()
		name := ""
		if clinic.Name != nil {
			name = *clinic.Name
		}

		fmt.Printf("%s %s\n", id, name)
	}
	fmt.Printf("Found %v clinics\n", len(list))

	return nil
}

func init() {
	clinicsCmd.AddCommand(clinicsListCmd)
	rootCmd.AddCommand(clinicsCmd)
}
