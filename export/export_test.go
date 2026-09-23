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
				formatted := export.FmtFloat(val, precision)
				Expect(formatted).To(Equal(expected))
			},
			Entry("0 precision", 2.45, 0, "2"),
			Entry("0 precision round to even downwards", 2.5, 0, "2"),
			Entry("0 precision round to even upwards", 3.5, 0, "4"),
			Entry("1 precision round upwards", 3.67, 1, "3.7"),
			Entry("1 precision round downwards", 3.44, 1, "3.4"),
			Entry("1 precision round to even upwards", 3.35, 1, "3.4"),
			Entry("1 precision round to even download", 8.65, 1, "8.6"),
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
		var connectionRequests patients.ProviderConnectionRequests
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
			dsCreatedTime := time.Date(2026, time.March, 9, 0, 0, 0, 0, time.UTC)
			pcrCreatedTime := dsCreatedTime.Add(-24 * time.Hour)
			pcrExpiredTime := pcrCreatedTime.Add(31 * 24 * time.Hour)
			connectionRequests = patients.ProviderConnectionRequests{
				"dexcom": patients.ConnectionRequests{
					{
						ProviderName:   "dexcom",
						CreatedTime:    pcrCreatedTime,
						ExpirationTime: pcrExpiredTime,
					},
				},
				"twiist": patients.ConnectionRequests{
					{
						ProviderName:   "twiist",
						CreatedTime:    pcrCreatedTime,
						ExpirationTime: pcrExpiredTime,
					},
				},
				"abbott": patients.ConnectionRequests{
					{
						ProviderName:   "abbott",
						CreatedTime:    pcrCreatedTime,
						ExpirationTime: pcrExpiredTime,
					},
				},
			}
			inactiveDataSource = patients.DataSource{
				ProviderName:   "dexcom",
				State:          "connected",
				CreatedTime:    timep(dsCreatedTime),
				LatestDataTime: timep(time.Date(2026, time.July, 9, 0, 0, 0, 0, time.UTC)),
			}
			connectedDataSource = patients.DataSource{
				ProviderName:   "twiist",
				State:          "connected",
				CreatedTime:    timep(time.Date(2026, time.March, 9, 0, 0, 0, 0, time.UTC)),
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
				FullName:                   strp("Some Patient"),
				UserId:                     &patientID,
				MRN:                        strp("123456789"),
				InvitedBy:                  &clinicianID,
				BirthDate:                  strp("2000-01-02"),
				Email:                      strp("patient@tidepool.org"),
				CreatedTime:                timep(time.Date(2003, time.April, 5, 12, 0, 0, 0, time.UTC)),
				Permissions:                &patients.Permissions{},
				DiagnosisType:              strp("type1"),
				DexcomDataSource:           &inactiveDataSource,
				TwiistDataSource:           &connectedDataSource,
				ProviderConnectionRequests: connectionRequests,
				ClinicSiteNames:            []string{"Site1", "Site2"},
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

		Describe("provider status column", func() {
			// Column index of the dexcom provider status in the CSV row.
			const dexcomStatusCol = 12

			now := time.Date(2026, time.July, 13, 19, 0, 0, 0, time.UTC)
			day := 24 * time.Hour
			ago := func(d time.Duration) time.Time { return now.Add(-d) }
			hence := func(d time.Duration) time.Time { return now.Add(d) }
			cr := func(created, expires time.Time) patients.ConnectionRequest {
				return patients.ConnectionRequest{
					ProviderName:   "dexcom",
					CreatedTime:    created,
					ExpirationTime: expires,
				}
			}
			ds := func(state string, created, latest *time.Time) *patients.DataSource {
				return &patients.DataSource{
					ProviderName:   "dexcom",
					State:          state,
					CreatedTime:    created,
					LatestDataTime: latest,
				}
			}
			modifiedAt := func(source *patients.DataSource,
				at time.Time) *patients.DataSource {

				source.ModifiedTime = &at
				return source
			}
			connectedAt := func(source *patients.DataSource,
				at time.Time) *patients.DataSource {

				source.ConnectedTime = &at
				return source
			}
			legacy := func(state string, modified *time.Time) *patients.DataSource {
				return &patients.DataSource{
					ProviderName: "dexcom",
					State:        state,
					ModifiedTime: modified,
				}
			}

			DescribeTable("dexcom",
				func(crs patients.ConnectionRequests, source *patients.DataSource,
					expected string) {

					if crs == nil {
						delete(patient.ProviderConnectionRequests, "dexcom")
					} else {
						patient.ProviderConnectionRequests["dexcom"] = crs
					}
					patient.DexcomDataSource = source
					params.ReportDate = now
					clinic := &clinics.Clinic{
						Id:               &clinicOID,
						PatientTags:      patientTags,
						PreferredBgUnits: "mg/dL",
					}
					e, err := export.NewPatientExportClinic(clinic, cs, nil, params)
					Expect(err).ToNot(HaveOccurred())
					row := e.ToCSVRow(patient)
					Expect(row[dexcomStatusCol]).To(Equal(expected))
				},

				// Neither a request nor a data source.
				Entry("no request and no data source", nil, nil, "NA"),

				// Connection request only.
				Entry("pending request only",
					patients.ConnectionRequests{cr(ago(day), hence(30*day))},
					nil, "pending"),
				Entry("expired request only",
					patients.ConnectionRequests{cr(ago(40*day), ago(10*day))},
					nil, "expired"),
				Entry("pending request without an expiration time",
					patients.ConnectionRequests{cr(ago(day), time.Time{})},
					nil, "pending"),
				Entry("expired request without an expiration time",
					patients.ConnectionRequests{cr(ago(40*day), time.Time{})},
					nil, "expired"),
				Entry("newest request wins when listed last",
					patients.ConnectionRequests{
						cr(ago(40*day), ago(10*day)),
						cr(ago(day), hence(30*day)),
					},
					nil, "pending"),
				Entry("newest request wins when listed first",
					patients.ConnectionRequests{
						cr(ago(day), hence(30*day)),
						cr(ago(40*day), ago(10*day)),
					},
					nil, "pending"),

				// Data source only.
				Entry("connected data source only",
					nil, ds("connected", timep(ago(10*day)), timep(ago(time.Hour))),
					"connected"),
				Entry("connected data source only without a created time",
					nil, ds("connected", nil, timep(ago(time.Hour))),
					"connected"),

				// Legacy data source without a created time: its modified time stands in.
				Entry("legacy data source modified after the request",
					patients.ConnectionRequests{cr(ago(20*day), hence(10*day))},
					legacy("disconnected", timep(ago(10*day))),
					"disconnected"),
				Entry("legacy data source modified before a pending request",
					patients.ConnectionRequests{cr(ago(day), hence(30*day))},
					legacy("disconnected", timep(ago(10*day))),
					"pending"),
				Entry("legacy data source modified before an expired request",
					patients.ConnectionRequests{cr(ago(35*day), ago(5*day))},
					legacy("error", timep(ago(40*day))),
					"expired"),
				Entry("legacy data source without created or modified times",
					patients.ConnectionRequests{cr(ago(day), hence(30*day))},
					legacy("connected", nil),
					"connected"),

				// Data source created after the newest request: data source wins.
				Entry("data source newer than pending request",
					patients.ConnectionRequests{cr(ago(20*day), hence(10*day))},
					ds("connected", timep(ago(10*day)), timep(ago(time.Hour))),
					"connected"),
				Entry("data source newer than expired request",
					patients.ConnectionRequests{cr(ago(40*day), ago(10*day))},
					ds("connected", timep(ago(5*day)), timep(ago(time.Hour))),
					"connected"),
				Entry("connected data source without data",
					patients.ConnectionRequests{cr(ago(20*day), hence(10*day))},
					ds("connected", timep(ago(10*day)), nil),
					"connected"),
				Entry("connected data source with stale data",
					patients.ConnectionRequests{cr(ago(20*day), hence(10*day))},
					ds("connected", timep(ago(10*day)), timep(ago(3*day))),
					"inactive"),
				Entry("disconnected data source",
					patients.ConnectionRequests{cr(ago(20*day), hence(10*day))},
					ds("disconnected", timep(ago(10*day)), timep(ago(time.Hour))),
					"disconnected"),
				Entry("data source with an error state",
					patients.ConnectionRequests{cr(ago(20*day), hence(10*day))},
					ds("error", timep(ago(10*day)), nil),
					"error"),
				Entry("data source with an empty state",
					patients.ConnectionRequests{cr(ago(20*day), hence(10*day))},
					ds("", timep(ago(10*day)), nil),
					"NA"),

				// Request created after a connected data source: the healthy connection
				// wins.
				Entry("pending request newer than connected data source",
					patients.ConnectionRequests{cr(ago(day), hence(30*day))},
					ds("connected", timep(ago(10*day)), timep(ago(time.Hour))),
					"connected"),
				Entry("expired request newer than connected data source",
					patients.ConnectionRequests{cr(ago(35*day), ago(5*day))},
					ds("connected", timep(ago(40*day)), timep(ago(time.Hour))),
					"connected"),

				// Request created after a data source that is not connected: request
				// wins, unless the source connected after it.
				Entry("pending request newer than disconnected data source",
					patients.ConnectionRequests{cr(ago(day), hence(30*day))},
					ds("disconnected", timep(ago(10*day)), nil),
					"pending"),
				Entry("expired request newer than errored data source",
					patients.ConnectionRequests{cr(ago(35*day), ago(5*day))},
					ds("error", timep(ago(40*day)), nil),
					"expired"),
				Entry("errored data source connected after the request",
					patients.ConnectionRequests{cr(ago(10*day), hence(20*day))},
					connectedAt(ds("error", timep(ago(40*day)), nil), ago(5*day)),
					"error"),
				Entry("disconnected data source connected before the request",
					patients.ConnectionRequests{cr(ago(10*day), hence(20*day))},
					connectedAt(ds("disconnected", timep(ago(40*day)), nil), ago(20*day)),
					"pending"),

				// Data source reconnected after the request: it keeps its created time,
				// but its connected time is reset.
				Entry("data source reconnected after the request",
					patients.ConnectionRequests{cr(ago(day), hence(30*day))},
					connectedAt(ds("connected", timep(ago(40*day)), timep(ago(time.Hour))),
						ago(time.Hour)),
					"connected"),
				Entry("disconnected data source modified after the request",
					patients.ConnectionRequests{cr(ago(day), hence(30*day))},
					modifiedAt(ds("disconnected", timep(ago(40*day)), nil), ago(time.Hour)),
					"pending"),
			)
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
