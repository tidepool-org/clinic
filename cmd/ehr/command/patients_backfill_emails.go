package command

import (
	"context"
	"fmt"
	"net/mail"
	"time"

	"github.com/spf13/cobra"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/redox"
	models "github.com/tidepool-org/clinic/redox_models"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

const minimumAgeSelfOwnedAccountYears = 13

var patientsBackfillEmailsParams = struct {
	BatchSize int
	ClinicId  string
	DryRun    bool
}{}

var patientsBackfillEmailsCommand = &cobra.Command{
	Use:   "backfill-emails {clinicId}",
	Args:  cobra.ExactArgs(1),
	Short: "Backfill custodial account emails from active Redox orders",
	Long:  "Backfill custodial account emails from active Redox orders",
	RunE: func(cmd *cobra.Command, args []string) error {
		patientsBackfillEmailsParams.ClinicId = args[0]
		return Run(backfillEmails)
	},
}

func init() {
	patientsBackfillEmailsCommand.Flags().BoolVar(&patientsBackfillEmailsParams.DryRun, "dry-run", false, "Only prints out users that will be updated")
	patientsBackfillEmailsCommand.Flags().IntVar(&patientsBackfillEmailsParams.BatchSize, "batch-size", 100, "Batch size to use when fetching users")

	patientsCmd.AddCommand(patientsBackfillEmailsCommand)
}

func backfillEmails(clinicsService clinics.Service, patientsService patients.Service, redoxService redox.Redox, logger *zap.SugaredLogger) error {
	clinic, err := getEHRClinic(patientsBackfillEmailsParams.ClinicId, clinicsService)
	if err != nil {
		return err
	}

	if clinic.EHRSettings.Provider != clinics.EHRProviderRedox {
		return fmt.Errorf("provider %s is not supported", clinic.EHRSettings.Provider)
	}

	hasEmail := false
	isCustodial := true

	updated := 0

	filter := patients.Filter{
		ClinicId:    &patientsBackfillEmailsParams.ClinicId,
		HasEmail:    &hasEmail,
		IsCustodial: &isCustodial,
	}
	sort := []*store.Sort{{
		Attribute: "_id",
		Ascending: true,
	}}
	page := store.DefaultPagination().WithLimit(patientsBackfillEmailsParams.BatchSize)
	for {
		result, err := patientsService.List(context.TODO(), &filter, page, sort)
		if err != nil {
			return fmt.Errorf("patients list error: %w", err)
		}

		for _, patient := range result.Patients {
			mrn := "(empty)"
			name := "(empty)"
			userId := "(empty)"

			if patient.Mrn != nil {
				mrn = *patient.Mrn
			}
			if patient.FullName != nil {
				name = *patient.FullName
			}
			if patient.UserId != nil {
				userId = *patient.UserId
			}
			var subscription *patients.EHRSubscription
			if patient.EHRSubscriptions != nil {
				if sub, exists := patient.EHRSubscriptions[patients.SubscriptionRedoxSummaryAndReports]; exists && sub.Active {
					subscription = &sub
				}
			}
			if subscription == nil {
				logger.Debugw("no active subscription found", "userId", userId)
				continue
			}

			messageRef := subscription.MatchedMessages[len(subscription.MatchedMessages)-1]
			message, err := redoxService.FindMessage(context.TODO(), messageRef.DocumentId.Hex(), messageRef.DataModel, messageRef.EventType)
			if err != nil {
				logger.Errorw("unable to find order document", "userId", userId, "documentId", messageRef.DocumentId.Hex(), "error", err)
				continue
			}
			var order models.NewOrder
			if err := bson.Unmarshal(message.Message, &order); err != nil {
				logger.Errorw("unable to unmarshal order for patient", "userId", userId, "error", err)
				continue
			}

			email, err := GetEmailAddressFromOrder(order)
			if err != nil {
				logger.Errorw("unable to get email address from order", "userId", userId, "error", err)
				continue
			}
			if email == nil {
				logger.Debugw("no email address found", "userId", userId)
				continue
			}

			if !patientsBackfillEmailsParams.DryRun {
				patient.Email = email

				_, err = patientsService.Update(context.TODO(), patients.PatientUpdate{
					ClinicId: patient.ClinicId.Hex(),
					UserId:   *patient.UserId,
					Patient:  *patient,
				})
			}

			if err != nil {
				logger.Errorw("error updating patient", "userId", userId, "error", err)
				continue
			}

			fmt.Printf("MRN %s - %s [%s] - New Email [%s]\n", mrn, name, userId, *email)
			updated++
		}

		if len(result.Patients) < page.Limit {
			break
		}
		page = page.WithOffset(page.Offset + page.Limit)
	}

	fmt.Printf("%v patients were updated\n", updated)
	return nil
}

func GetBirthDateFromOrder(order models.NewOrder) (*time.Time, error) {
	if order.Patient.Demographics == nil || order.Patient.Demographics.DOB == nil {
		return nil, fmt.Errorf("date of birth is missing")
	}

	parsed, err := time.Parse(time.DateOnly, *order.Patient.Demographics.DOB)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func GetEmailAddressFromOrder(order models.NewOrder) (*string, error) {
	birthDate, err := GetBirthDateFromOrder(order)
	if err != nil {
		return nil, err
	}

	var email *string
	if shouldUseGuarantorEmail(*birthDate) {
		email, err = GetGuarantorEmailAddressFromOrder(order)
		if err != nil {
			return nil, err
		}
	} else {
		email, err = GetPatientEmailAddressFromOrder(order)
		if err != nil {
			return nil, err
		}
	}

	if email == nil {
		return nil, nil
	}

	addr, err := mail.ParseAddress(*email)
	if err != nil {
		return nil, fmt.Errorf("email address is invalid")
	}

	return &addr.Address, nil
}

func shouldUseGuarantorEmail(birthDate time.Time) bool {
	now := time.Now()
	cutoff := birthDate.AddDate(minimumAgeSelfOwnedAccountYears, 0, 0)
	return !cutoff.Before(now)
}

func GetPatientEmailAddressFromOrder(order models.NewOrder) (*string, error) {
	if order.Patient.Demographics == nil || order.Patient.Demographics.EmailAddresses == nil || len(*order.Patient.Demographics.EmailAddresses) == 0 {
		return nil, nil
	}

	email, ok := (*order.Patient.Demographics.EmailAddresses)[0].(string)
	if !ok {
		return nil, fmt.Errorf("patient email address is not a string")
	}

	return &email, nil
}

func GetGuarantorEmailAddressFromOrder(order models.NewOrder) (*string, error) {
	if order.Visit == nil || order.Visit.Guarantor == nil || order.Visit.Guarantor.EmailAddresses == nil || len(*order.Visit.Guarantor.EmailAddresses) == 0 {
		return nil, nil
	}

	email, ok := (*order.Visit.Guarantor.EmailAddresses)[0].(string)
	if !ok {
		return nil, fmt.Errorf("guarantor email address is not a string")
	}

	return &email, nil
}
