package xealth

import (
	"fmt"
	"github.com/tidepool-org/clinic/patients"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"slices"
	"strings"
	"time"
)

const (
	ProgramTitle = "Tidepool"
)

func GetProgramDescription(lastUpload time.Time, lastViewed time.Time, permissions *patients.Permissions, dataSources *[]patients.DataSource) *string {
	items := []string{
		fmt.Sprintf("Last Upload: %s", formatDateForDescription(lastUpload)),
		fmt.Sprintf("Last Viewed by You: %s", formatDateForDescription(lastViewed)),
		fmt.Sprintf("Claimed Account?: %s", formatBoolean(permissions.IsClaimed())),
		fmt.Sprintf("Cloud Connections: %s", GetCloudConnections(dataSources)),
	}
	description := strings.Join(items, " | ")
	return &description
}

func GetCloudConnections(dataSources *[]patients.DataSource) string {
	// build a map with the most recently modified data sources for each provider
	mostRecentDataSourceByProvider := map[string]patients.DataSource{}
	if dataSources != nil {
		for _, dataSource := range *dataSources {
			newModifiedTime := time.Time{}
			if dataSource.ModifiedTime != nil {
				newModifiedTime = *dataSource.ModifiedTime
			}
			existingModifiedTime := time.Time{}
			if existing, ok := mostRecentDataSourceByProvider[dataSource.ProviderName]; ok {
				if existing.ModifiedTime != nil {
					existingModifiedTime = *existing.ModifiedTime
				}
			}
			if existingModifiedTime.IsZero() || newModifiedTime.After(existingModifiedTime) {
				mostRecentDataSourceByProvider[dataSource.ProviderName] = dataSource
			}
		}
	}

	result := make([]string, 0, len(mostRecentDataSourceByProvider))
	for _, dataSource := range mostRecentDataSourceByProvider {
		result = append(result, fmt.Sprintf("%s (%s)", formatDataSourceProviderName(dataSource.ProviderName), formatDataSourceState(dataSource.State)))
	}
	slices.Sort(result)

	if len(result) == 0 {
		return "None"
	}

	return strings.Join(result, ", ")
}

func formatDataSourceState(state string) string {
	if state == patients.DataSourceStatePendingReconnect {
		return "pending reconnect"
	}
	return strings.ToLower(state)
}

func formatDataSourceProviderName(name string) string {
	if name == patients.TwiistDataSourceProviderName {
		return strings.ToLower(name)
	}
	return cases.Title(language.English, cases.Compact).String(name)
}

func formatDateForDescription(date time.Time) string {
	if date.IsZero() {
		return "N/A"
	}
	return date.Format(time.DateOnly)
}

func formatBoolean(value bool) string {
	if value {
		return "Yes"
	}

	return "No"
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
