package command

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"strings"
)

var patientsListParams = struct {
	Limit                   int
	Offset                  int
	PatientId               string
	ClinicId                string
	OnlyWithSubscription    bool
	OnlyWithoutSubscription bool
	OnlyWithMRN             bool
	OnlyWithoutMRN          bool
}{}

var patientsListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.ExactArgs(1),
	Short: "List Xealth Enabled Clinics",
	Long:  "The list command is used to retrieve a list of all Xealth enabled clinics",
	RunE: func(cmd *cobra.Command, args []string) error {
		patientsListParams.ClinicId = args[0]
		return Run(listPatients)
	},
}

func listPatients(clinicsService clinics.Service, patientsService patients.Service) error {
	_, err := getXealthClinic(patientsListParams.ClinicId, clinicsService)
	if err != nil {
		return err
	}

	page := store.DefaultPagination().
		WithLimit(patientsListParams.Limit).
		WithOffset(patientsListParams.Offset)
	sort := []*store.Sort{{
		Attribute: "mrn",
		Ascending: true,
	}}
	filter := patients.Filter{
		ClinicId: &patientsListParams.ClinicId,
	}
	if patientsListParams.OnlyWithSubscription {
		hasSubscription := true
		filter.HasSubscription = &hasSubscription
	} else if patientsListParams.OnlyWithoutSubscription {
		hasSubscription := false
		filter.HasSubscription = &hasSubscription
	}
	if patientsListParams.OnlyWithMRN {
		hasMRN := true
		filter.HasMRN = &hasMRN
	} else if patientsListParams.OnlyWithoutMRN {
		hasMRN := false
		filter.HasMRN = &hasMRN
	}
	if patientsListParams.PatientId != "" {
		filter.UserId = &patientsListParams.PatientId
	}

	result, err := patientsService.List(context.TODO(), &filter, page, sort)
	for _, patient := range result.Patients {
		mrn := "(empty)"
		name := "(empty)"
		userId := "(empty)"

		subscriptions := make([]string, 0, len(patient.EHRSubscriptions))
		if patient.Mrn != nil {
			mrn = *patient.Mrn
		}
		if patient.FullName != nil {
			name = *patient.FullName
		}
		if patient.UserId != nil {
			userId = *patient.UserId
		}
		for _, v := range patient.EHRSubscriptions {
			active := "inactive"
			if v.Active == true {
				active = "active"
			}

			subscriptions = append(subscriptions, fmt.Sprintf("%s (%s)", v.Provider, active))
		}

		fmt.Printf("MRN %s - %s [%s] - Subscriptions [%s]\n", mrn, name, userId, strings.Join(subscriptions, ", "))
	}

	fmt.Printf("Found %v patients\n", result.MatchingCount)

	return nil
}

func init() {
	patientsListCmd.Flags().IntVarP(&patientsListParams.Limit, "limit", "l", 20, "The number of patients to display")
	patientsListCmd.Flags().IntVarP(&patientsListParams.Offset, "offset", "o", 0, "The number of patients to skip")
	patientsListCmd.Flags().StringVar(&patientsListParams.PatientId, "user-id", "", "The user id of the patient")
	patientsListCmd.Flags().BoolVar(&patientsListParams.OnlyWithSubscription, "with-subscription", false, "Return only users with EHR subscriptions")
	patientsListCmd.Flags().BoolVar(&patientsListParams.OnlyWithSubscription, "without-subscription", false, "Return only users without EHR subscriptions")
	patientsListCmd.Flags().BoolVar(&patientsListParams.OnlyWithMRN, "with-mrn", false, "Return only users with MRN")
	patientsListCmd.Flags().BoolVar(&patientsListParams.OnlyWithoutMRN, "without-mrn", false, "Return only users without MRN")

	patientsCmd.AddCommand(patientsListCmd)
	rootCmd.AddCommand(patientsCmd)
}
