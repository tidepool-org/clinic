package export_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/export"
	"github.com/tidepool-org/clinic/patients"
)

var _ = Describe("Export", func() {
	Describe("ToCSVRow", func() {
		It("matches expected columns", func() {
			clinicianID := "a578d15f-73b6-4e25-9295-95707fca3520"
			patientTagID := "6a5794ac4762b6efdb48aaaa"
			patientTagOID, _ := primitive.ObjectIDFromHex(patientTagID)
			clinicID := "6a5794ac4762b6efdb48aaab"
			clinicOID, _ := primitive.ObjectIDFromHex(clinicID)
			patientID := "7ce10e4c-8930-4770-8323-c8305db09c61"
			reportDate := time.Date(2026, time.July, 13, 19, 0, 0, 0, time.UTC)

			inactiveDataSource := patients.DataSource{
				ProviderName:   "dexcom",
				State:          "connected",
				LatestDataTime: timep(time.Date(2026, time.July, 9, 0, 0, 0, 0, time.UTC)),
			}
			expiredDataSource := patients.DataSource{
				ExpirationTime: timep(time.Date(2025, time.January, 2, 3, 0, 0, 0, time.UTC)),
				ProviderName:   "abbott",
				State:          "pending",
			}
			connectedDataSource := patients.DataSource{
				ProviderName:   "twiist",
				State:          "connected",
				LatestDataTime: timep(time.Date(2026, time.July, 12, 20, 0, 0, 0, time.UTC)),
			}
			patientTags := []clinics.PatientTag{
				{
					Id:       &patientTagOID,
					Name:     "Some Tag",
					Patients: 1,
				},
			}
			cs := []*clinicians.Clinician{
				{
					UserId: &clinicianID,
					Name:   strp("Some Clinician"),
				},
			}
			clinic := &clinics.Clinic{
				Id:          &clinicOID,
				PatientTags: patientTags,
			}
			params := patients.ExportParams{
				Period:              "1d",
				ExporterClinicianID: clinicianID,
				WorkspaceID:         clinicID,
				ReportDate:          reportDate,
			}
			patient := &patients.ExportedPatient{
				FullName:         strp("Some Patient"),
				UserId:           &patientID,
				MRN:              strp("123456789"),
				InvitedBy:        &clinicianID,
				BirthDate:        strp("2000-01-02"),
				Email:            strp("patient@tidepool.org"),
				CreatedTime:      timep(time.Date(2003, time.April, 5, 12, 0, 0, 0, time.UTC)),
				Permissions:      &patients.Permissions{},
				DiagnosisType:    strp("type1"),
				DexcomDataSource: &inactiveDataSource,
				AbbottDataSource: &expiredDataSource,
				TwiistDataSource: &connectedDataSource,
				ClinicSiteNames:  []string{"Site1", "Site2"},
				TagIds: &[]string{
					patientTagID,
					"nameless-tag-id",
				},
				CgmLastDataDate:     timep(time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)),
				CgmActiveWearTime:   floatp(0.71428),
				CgmDaysWithData:     intp(13),
				CgmHoursWithData:    intp(238),
				CgmAverageGlucose:   floatp(5.131),
				CgmTimeInLevel2Hypo: floatp(0.02134),
				CgmTimeInLevel1Hypo: floatp(0.06013),
				CgmTimeInTarget:     floatp(0.82333),
				BgmLastDataDate:     timep(time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)),
				BgmAverageGlucose:   floatp(6.231),
				BgmTotalReadings:    intp(5),
				BgmReadingsPerDay:   floatp(2.5),
				BgmLowEvents:        intp(3),
				BgmHighEvents:       intp(1),
			}

			e, err := export.NewPatientExportClinic(clinic, cs, nil, params)
			Expect(err).ToNot(HaveOccurred())
			row := e.ToCSVRow(patient)
			expectedRow := []string{
				"Some Patient",
				patientID,
				"123456789",
				"2000-01-02",
				"patient@tidepool.org",
				"Claimed",
				"2003-04-05",
				"Some Clinician",
				"Site1,Site2",
				`Some Tag,`,
				"",
				"type1",
				"inactive",
				"2026-07-09",
				"expired",
				"",
				"connected",
				"2026-07-12",
				"2026-07-13",
				"71",
				"13",
				"238",
				"92",
				"",
				"",
				"",
				"2",
				"6",
				"82",
				"",
				"",
				"2026-07-10",
				"112",
				"2",
				"5",
				"3",
				"1",
			}
			Expect(row).To(Equal(expectedRow))
		})
	})
})

func strp(s string) *string {
	return &s
}

func floatp(f float64) *float64 {
	return &f
}

func timep(t time.Time) *time.Time {
	return &t
}

func intp(i int) *int {
	return &i
}
