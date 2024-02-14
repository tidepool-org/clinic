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
		fmt.Sprintf("Last Upload: %s", formatDateForDescription(lastUpload)),
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

func GetSummaryLastUpdatedDate(patient *patients.Patient) (result time.Time) {
	if patient != nil && patient.Summary != nil {
		result = patient.Summary.GetLastUpdatedDate()
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
	return &order.EventNotification.ProgramId
}

func GetProgramTitle() *string {
	title := ProgramTitle
	return &title
}

func HasStatusView(patient *patients.Patient, subscription *patients.EHRSubscription) *bool {
	result := false
	if subscription != nil {
		date := GetLastUploadDate(patient)
		result = subscription.Active && !date.IsZero()
	}

	return &result
}

func IsProgramAlertActive(lastUpload time.Time, lastViewed time.Time) *bool {
	active := lastUpload.After(lastViewed)
	return &active
}
