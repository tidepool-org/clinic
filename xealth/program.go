package xealth

import (
	"fmt"
	"github.com/tidepool-org/clinic/patients"
	"strings"
	"time"
)

const (
	ProgramTitle = "Tidepool"
)

func GetProgramDescription(lastUpload time.Time, lastViewed time.Time) *string {
	items := []string{
		fmt.Sprintf("Last Upload Date: %s", formatDateForDescription(lastUpload)),
		fmt.Sprintf("Last Viewed by You: %s", formatDateForDescription(lastViewed)),
	}
	description := strings.Join(items, " | ")
	return &description
}

func formatDateForDescription(date time.Time) string {
	if date.IsZero() {
		return "N/A"
	}
	return date.Format(time.DateOnly)
}

func GetLastUploadDate(patient *patients.Patient) (result time.Time) {
	if patient != nil && patient.Summary != nil {
		result = patient.Summary.GetLastUploadDate()
	}
	return
}

func GetProgramEnrollmentDateFromOrder(order *OrderEvent) *string {
	if order == nil {
		return nil
	}

	date := order.EventNotification.EventTimeStamp.Format(time.DateOnly)
	return &date
}

func GetProgramIdFromOrder(order *OrderEvent) *string {
	if order == nil {
		return nil
	}
	return &order.EventNotification.OrderId
}

func GetProgramTitle() *string {
	title := ProgramTitle
	return &title
}

func IsSubscriptionActive(subscription *patients.EHRSubscription) *bool {
	if subscription == nil {
		return nil
	}

	return &subscription.Active
}

func IsProgramAlertActive(lastUpload time.Time, lastViewed time.Time) *bool {
	active := lastUpload.After(lastViewed)
	return &active
}
