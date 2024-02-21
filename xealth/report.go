package xealth

import (
	"fmt"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"net/url"
	"time"
)

const (
	defaultReportingPeriod = time.Hour * 24 * 14
)

func GeneratePDFViewerUrl(baseUrl string, token string, patient patients.Patient, clinic clinics.Clinic) (*url.URL, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}
	u = u.JoinPath("v1", "xealth", "report", "web", "viewer.html")

	query := u.Query()
	query.Add(restrictedTokenKey, token)
	query.Add("patientId", *patient.UserId)
	query.Add("clinicId", clinic.Id.Hex())
	u.RawQuery = query.Encode()

	return u, nil
}

func GenerateReportUrl(baseUrl string, token string, patient patients.Patient, clinic clinics.Clinic) (*url.URL, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	if patient.UserId == nil {
		return u, fmt.Errorf("userId is required")
	}
	if patient.BirthDate == nil {
		return u, fmt.Errorf("birth date is required")
	}
	if patient.Mrn == nil {
		return u, fmt.Errorf("mrn is required")
	}
	if patient.FullName == nil {
		return u, fmt.Errorf("full name is required")
	}

	u = u.JoinPath("export", "report", *patient.UserId)

	query := u.Query()
	query.Add("reports", "all")
	query.Add("inline", "true")

	query.Add("userId", *patient.UserId)
	query.Add("dob", *patient.BirthDate)
	query.Add("mrn", *patient.Mrn)
	query.Add("fullName", *patient.FullName)

	if clinic.Timezone != nil {
		query.Add("tzName", *clinic.Timezone)
	}
	if clinic.PreferredBgUnits != "" {
		query.Add("bgUnits", clinic.PreferredBgUnits)
	}

	endDate := getReportEndDate(patient)
	startDate := getReportStartDate(endDate, defaultReportingPeriod)
	if !startDate.IsZero() {
		query.Add("startDate", startDate.Format(time.RFC3339))
	}
	if !endDate.IsZero() {
		query.Add("endDate", endDate.Format(time.RFC3339))
	}

	query.Add(restrictedTokenKey, token)
	u.RawQuery = query.Encode()
	return u, nil
}

func getReportEndDate(patient patients.Patient) (endDate time.Time) {
	if patient.Summary != nil {
		if patient.Summary.CGM != nil && patient.Summary.CGM.Dates.LastData != nil {
			endDate = *patient.Summary.CGM.Dates.LastData
		}
		if patient.Summary.BGM != nil && patient.Summary.BGM.Dates.LastData != nil {
			if patient.Summary.BGM.Dates.LastData.After(endDate) {
				endDate = *patient.Summary.BGM.Dates.LastData
			}
		}
	}
	return
}

func getReportStartDate(endDate time.Time, period time.Duration) (startDate time.Time) {
	if !endDate.IsZero() {
		startDate = endDate.Add(-period)
	}
	return
}
