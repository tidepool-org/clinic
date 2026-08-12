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
	Describe("Format and conversion functions", func() {
		DescribeTable("FmtFloat",
			func(val float64, precision int, expected string) {

			},
			Entry("0 precision", 2.45, 0, "2"),
			Entry("0 precision round to even downwards", 2.5, 0, "2"),
			Entry("0 precision round to even upwards", 3.5, 0, "3"),
			Entry("1 precision round upwards", 3.67, 1, "3.7"),
			Entry("1 precision round downwards", 3.44, 1, "3.4"),
			Entry("1 precision round to even upwards", 3.5, 1, "4.0"),
			Entry("1 precision round to even download", 4.5, 1, "4.0"),
		)
	})

	Describe("ToCSVRow", func() {
		var clinicianID string
		var patientTagID string
		var patientTagOID primitive.ObjectID
		var clinicID string
		var clinicOID primitive.ObjectID
		var patientID string
		var reportDate time.Time
		var inactiveDataSource patients.DataSource
		var expiredDataSource patients.DataSource
		var connectedDataSource patients.DataSource
		var patientTags []clinics.PatientTag
		var cs []*clinicians.Clinician
		var params patients.ExportParams
		var patient *patients.ExportedPatient

		BeforeEach(func() {
			clinicianID = "a578d15f-73b6-4e25-9295-95707fca3520"
			patientTagID = "6a5794ac4762b6efdb48aaaa"
			patientTagOID, _ = primitive.ObjectIDFromHex(patientTagID)
			clinicID = "6a5794ac4762b6efdb48aaab"
			clinicOID, _ = primitive.ObjectIDFromHex(clinicID)
			patientID = "7ce10e4c-8930-4770-8323-c8305db09c61"
			reportDate = time.Date(2026, time.July, 13, 19, 0, 0, 0, time.UTC)

			inactiveDataSource = patients.DataSource{
				ProviderName:   "dexcom",
				State:          "connected",
				LatestDataTime: timep(time.Date(2026, time.July, 9, 0, 0, 0, 0, time.UTC)),
			}
			expiredDataSource = patients.DataSource{
				ExpirationTime: timep(time.Date(2025, time.January, 2, 3, 0, 0, 0, time.UTC)),
				ProviderName:   "abbott",
				State:          "pending",
			}
			connectedDataSource = patients.DataSource{
				ProviderName:   "twiist",
				State:          "connected",
				LatestDataTime: timep(time.Date(2026, time.July, 12, 20, 0, 0, 0, time.UTC)),
			}
			patientTags = []clinics.PatientTag{
				{
					Id:       &patientTagOID,
					Name:     "Some Tag",
					Patients: 1,
				},
			}
			cs = []*clinicians.Clinician{
				{
					UserId: &clinicianID,
					Name:   strp("Some Clinician"),
				},
			}
			params = patients.ExportParams{
				Period:              "1d",
				ExporterClinicianID: clinicianID,
				WorkspaceID:         clinicID,
				ReportDate:          reportDate,
			}
			patient = &patients.ExportedPatient{
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
				CgmStdDev:           floatp(2.3),
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
		})

		It("matches preferred units of mmol/L", func() {
			clinic := &clinics.Clinic{
				Id:               &clinicOID,
				PatientTags:      patientTags,
				PreferredBgUnits: "mmol/L",
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
				"2026-07-13",         // cgm last data date
				"71.428",             // cgm active wear time
				"13",                 // cgm days w/ data
				"238",                // cgm hours w/ data
				"92",                 // avg glucose mg/dL
				"",                   // cgm gmi %
				"2.3",                // cgm stdev in clnic preferred units
				"",                   // cbm cv %
				"2.1340000000000003", // time in level 2 hypo %
				"6.013",              //  time in level 1 hypo %
				"82.333",             // cgm time in target %
				"",                   // cgm time in level 1 hyper %
				"",                   //  cgm time in level 2 hyper %
				"2026-07-10",         // bgm last data date
				"112",                // bgm avg glucose mg/dL
				"2",                  // bgm readings / day
				"5",                  // bgm total readings
				"3",                  // bgm # low events
				"1",                  // bgm # high events
			}
			Expect(row).To(Equal(expectedRow))
		})

		It("matches preferred units of mg/dL", func() {
			clinic := &clinics.Clinic{
				Id:               &clinicOID,
				PatientTags:      patientTags,
				PreferredBgUnits: "mg/dL",
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
				"2026-07-13",         // cgm last data date
				"71.428",             // cgm active wear time
				"13",                 // cgm days w/ data
				"238",                // cgm hours w/ data
				"92",                 // avg glucose mg/dL
				"",                   // cgm gmi %
				"41.4",               // cgm stdev in clnic preferred units
				"",                   // cbm cv %
				"2.1340000000000003", // time in level 2 hypo %
				"6.013",              //  time in level 1 hypo %
				"82.333",             // cgm time in target %
				"",                   // cgm time in level 1 hyper %
				"",                   //  cgm time in level 2 hyper %
				"2026-07-10",         // bgm last data date
				"112",                // bgm avg glucose mg/dL
				"2",                  // bgm readings / day
				"5",                  // bgm total readings
				"3",                  // bgm # low events
				"1",                  // bgm # high events
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
