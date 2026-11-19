package xealth

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/sites"
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

func GenerateReportUrl(baseUrl string, token string, patient patients.Patient, clinic clinics.Clinic, siteChangeSource string) (*url.URL, error) {
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

	switch patient.GlycemicRanges.Type {
	case patients.GlycemicRangeTypeCustom:
		query.Add("glycemicRangeType", string(patients.GlycemicRangeTypeCustom))
		for _, threshold := range patient.GlycemicRanges.Custom.Thresholds {
			name := &bytes.Buffer{}
			w := csv.NewWriter(name)
			w.Write([]string{threshold.Name})
			w.Flush()
			query.Add("glycemicRangeThresholds", fmt.Sprintf("name,%s,upperBound.value,%f,upperBound.units,%s,inclusive,%t", strings.TrimSpace(name.String()), threshold.UpperBound.Value, threshold.UpperBound.Units, threshold.Inclusive))
		}
	case patients.GlycemicRangeTypePreset:
		query.Add("glycemicRangeType", string(patients.GlycemicRangeTypePreset))
		query.Add("glycemicRangePreset", string(patient.GlycemicRanges.Preset))
	}

	for _, name := range getPatientTagNames(patient, clinic) {
		query.Add("patientTags", name)
	}
	for _, name := range getSiteNames(patient, clinic) {
		query.Add("sites", name)
	}

	switch siteChangeSource {
	case "cannulaPrime", "reservoirChange", "tubingPrime":
		query.Add("siteChangeSource", siteChangeSource)
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

// getPatientTagNames returns all patient tag names, dropping any that no
// longer exist on the clinic.
func getPatientTagNames(patient patients.Patient, clinic clinics.Clinic) []string {
	if patient.Tags == nil {
		return nil
	}
	var names []string
	for _, tag := range clinic.PatientTags {
		if tag.Id != nil && tag.Name != "" && slices.Contains(*patient.Tags, *tag.Id) {
			names = append(names, tag.Name)
		}
	}
	return names
}

// getSiteNames returns all patient site names, dropping any that no longer
// exist on the clinic.
func getSiteNames(patient patients.Patient, clinic clinics.Clinic) []string {
	if patient.Sites == nil {
		return nil
	}
	var names []string
	for _, site := range clinic.Sites {
		if site.Name != "" && slices.ContainsFunc(*patient.Sites, func(s sites.Site) bool { return s.Id == site.Id }) {
			names = append(names, site.Name)
		}
	}
	return names
}
