package command

import (
	"context"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/cobra"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/xealth"
	"github.com/tidepool-org/clinic/xealth_client"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
)

var patientsEnrollParams = struct {
	Limit     int
	PatientId string
	ClinicId  string
	MRNOrigin string
	DryRun    bool
}{}

var patientsEnrollCmd = &cobra.Command{
	Use:   "enroll",
	Args:  cobra.ExactArgs(1),
	Short: "Enroll Patients in Xealth",
	Long:  "The enroll command is used to place an enrollment order in Xealth",
	RunE: func(cmd *cobra.Command, args []string) error {
		patientsEnrollParams.ClinicId = args[0]
		return Run(enrollPatients, fx.Provide(NewXealthClient))
	},
}

func enrollPatients(clinicsService clinics.Service, patientsService patients.Service, xealthClient xealth_client.ClientWithResponsesInterface) error {
	clinic, err := getXealthClinic(patientsEnrollParams.ClinicId, clinicsService)
	if err != nil {
		return err
	}

	page := store.DefaultPagination().WithLimit(patientsEnrollParams.Limit)
	sort := []*store.Sort{{
		Attribute: "mrn",
		Ascending: true,
	}}

	hasMRN := true
	HasSubscription := false
	filter := patients.Filter{
		ClinicId:        &patientsEnrollParams.ClinicId,
		HasMRN:          &hasMRN,
		HasSubscription: &HasSubscription,
	}
	if patientsEnrollParams.PatientId != "" {
		filter.UserId = &patientsEnrollParams.PatientId
	}

	result, err := patientsService.List(context.TODO(), &filter, page, sort)
	if err != nil {
		return err
	}

	programId := clinic.EHRSettings.ProcedureCodes.CreateAccountAndEnableReports
	if programId == nil {
		return fmt.Errorf("cannot enroll patient because program id is not set in ehr settings")
	}

	fmt.Printf("Enrolling %v patients out of %v total\n", len(result.Patients), result.MatchingCount)

	for _, patient := range result.Patients {
		if patient.Mrn == nil || *patient.Mrn == "" {
			fmt.Printf("Cannot enroll patient (userId: %s) with because mrn is not set\n", *patient.UserId)
			continue
		}

		req := xealth_client.PostPartnerWriteOrderDeploymentJSONRequestBody{
			OrderType:    "enrollment",
			PartnerId:    "tidepool",
			ProgramId:    *programId,
			ProgramTitle: "Tidepool",
		}
		req.PatientId.Id = *patient.Mrn
		req.PatientId.Type = clinic.EHRSettings.GetMrnIDType()
		req.PatientId.Origin = xealth_client.PostPartnerWriteOrderDeploymentJSONBodyPatientIdOrigin(patientsEnrollParams.MRNOrigin)

		fmt.Printf("Enrolling patient with MRN %s\n", *patient.Mrn)
		response, err := xealthClient.PostPartnerWriteOrderDeploymentWithResponse(
			context.TODO(),
			clinic.EHRSettings.SourceId,
			nil,
			req,
		)
		if err != nil {
			return err
		}
		if response.StatusCode() != http.StatusOK {
			fmt.Printf("Unable to enroll patient: %v\n", string(response.Body))
		}
	}

	return nil
}

func init() {
	patientsEnrollCmd.Flags().IntVarP(&patientsEnrollParams.Limit, "limit", "l", 20, "The max number of patients to enroll")
	patientsEnrollCmd.Flags().StringVar(&patientsEnrollParams.PatientId, "user-id", "", "The user id of the patient")
	patientsEnrollCmd.Flags().StringVar(&patientsEnrollParams.MRNOrigin, "mrn-origin", "cerner", "The mrn origin to use when creating the order")
	patientsEnrollCmd.Flags().BoolVar(&patientsEnrollParams.DryRun, "dry-run", false, "The mrn origin to use when creating the order")

	patientsCmd.AddCommand(patientsEnrollCmd)
	rootCmd.AddCommand(patientsCmd)
}

func NewXealthClient(logger *zap.SugaredLogger) (xealth_client.ClientWithResponsesInterface, error) {
	clientConfig := &xealth.Config{}
	if err := envconfig.Process("", clientConfig); err != nil {
		return nil, err
	}

	return xealth.NewClient(clientConfig, logger)
}
