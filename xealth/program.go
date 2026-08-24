package xealth

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/tidepool-org/clinic/patients"
)

const (
	ProgramTitle = "Tidepool"
)

func GetProgramDescription(lastUpload time.Time, lastViewed time.Time, patient *patients.Patient) *string {
	items := []string{
		fmt.Sprintf("Last Upload: %s", formatDateForDescription(lastUpload)),
		fmt.Sprintf("Last Viewed by You: %s", formatDateForDescription(lastViewed)),
		fmt.Sprintf("Claimed Account?: %s", formatBoolean(patient.Permissions.IsClaimed())),
		fmt.Sprintf("Cloud Connections: %s", GetCloudConnections(patient)),
	}
	description := strings.Join(items, " | ")
	return &description
}

type connection struct {
	DataSource        *patients.DataSource
	ConnectionRequest *patients.ConnectionRequest
}

func GetCloudConnections(patient *patients.Patient) string {
	// build a map with the most recently modified data sources for each provider
	mostRecentByProvider := map[string]connection{}
	if patient.DataSources != nil {
		for _, dataSource := range *patient.DataSources {
			newModifiedTime := time.Time{}
			if dataSource.ModifiedTime != nil {
				newModifiedTime = *dataSource.ModifiedTime
			}
			existingModifiedTime := time.Time{}
			if existing, ok := mostRecentByProvider[dataSource.ProviderName]; ok {
				if existing.DataSource.ModifiedTime != nil {
					existingModifiedTime = *existing.DataSource.ModifiedTime
				}
			}
			if existingModifiedTime.IsZero() || newModifiedTime.After(existingModifiedTime) {
				mostRecentByProvider[dataSource.ProviderName] = connection{
					DataSource: &dataSource,
				}
			}
		}
	}

	for providerName, pcrs := range patient.ProviderConnectionRequests {
		if len(pcrs) == 0 {
			continue
		}
		pcr := pcrs[0]

		var conn connection
		var found bool
		if conn, found = mostRecentByProvider[providerName]; !found {
			conn = connection{}
		}

		conn.ConnectionRequest = &pcr
		mostRecentByProvider[providerName] = conn
	}

	result := make([]string, 0, len(mostRecentByProvider))
	for providerName, conn := range mostRecentByProvider {
		formattedName := formatDataSourceProviderName(providerName)
		formattedState := formatDataSourceState(conn)
		result = append(result, fmt.Sprintf("%s (%s)", formattedName, formattedState))
	}
	slices.Sort(result)

	if len(result) == 0 {
		return "None"
	}

	return strings.Join(result, ", ")
}

func formatDataSourceState(conn connection) string {
	if conn.DataSource == nil {
		return "pending"
	}
	if conn.DataSource.ModifiedTime != nil &&
		conn.ConnectionRequest.CreatedTime.After(*conn.DataSource.ModifiedTime) {
		return "pending reconnect"
	}
	return strings.ToLower(conn.DataSource.State)
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
