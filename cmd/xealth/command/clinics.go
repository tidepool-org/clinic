package command

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/store"
)

var clinicsCmd = &cobra.Command{
	Use:   "clinics",
	Short: "Xealth Clinics",
	Long:  "The clinics command is used to manage Xealth clinics",
}

func init() {
	rootCmd.AddCommand(clinicsCmd)
}

func getXealthClinic(clinicId string, clinicsService clinics.Service) (*clinics.Clinic, error) {
	enabled := true
	provider := clinics.EHRProviderXealth
	page := store.DefaultPagination().WithLimit(1)
	list, err := clinicsService.List(context.TODO(), &clinics.Filter{
		Ids:         []string{clinicId},
		EHREnabled:  &enabled,
		EHRProvider: &provider,
	}, page)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("the clinic is not configured to use Xealth")
	}

	return list[0], nil
}
