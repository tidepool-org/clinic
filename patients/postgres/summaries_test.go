package postgres_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/patients"
)

func f64(v float64) *float64 { return &v }
func intp(v int) *int        { return &v }

var _ = Describe("Patient Summaries Writer", func() {
	newSummary := func(cgmId, bgmId string, lastData time.Time) *patients.Summary {
		return &patients.Summary{
			CGM: &patients.PatientCGMStats{
				Id: cgmId,
				Config: patients.PatientSummaryConfig{
					SchemaVersion:            2,
					HighGlucoseThreshold:     10,
					LowGlucoseThreshold:      3.9,
					VeryHighGlucoseThreshold: 13.9,
					VeryLowGlucoseThreshold:  3,
				},
				Dates: patients.PatientSummaryDates{
					LastData:    &lastData,
					HasLastData: true,
				},
				Periods: patients.PatientCGMPeriods{
					"14d": {
						TimeInTargetPercent:        f64(0.72),
						HasTimeInTargetPercent:     true,
						TotalRecords:               intp(1234),
						HasTotalRecords:            true,
						TimeCGMUsePercent:          f64(0.9),
						GlucoseManagementIndicator: f64(6.5),
						DaysWithData:               14,
						HoursWithData:              300,
					},
					"30d": {TimeInTargetPercent: f64(0.65), HasTimeInTargetPercent: true},
				},
			},
			BGM: &patients.PatientBGMStats{
				Id: bgmId,
				Dates: patients.PatientSummaryDates{
					LastData:    &lastData,
					HasLastData: true,
				},
				Periods: patients.PatientBGMPeriods{
					"14d": {AverageGlucoseMmol: f64(7.7), TotalRecords: intp(55)},
				},
			},
		}
	}

	It("snapshots and replaces summary rows with the patient", func() {
		writer, conn, ctx := newTestWriter()
		patient := randomPatient(primitive.NewObjectID())
		cgmId := primitive.NewObjectID().Hex()
		bgmId := primitive.NewObjectID().Hex()
		lastData := time.Now().UTC().Truncate(time.Millisecond)
		patient.Summary = newSummary(cgmId, bgmId, lastData)

		Expect(writer.UpsertPatient(ctx, patient)).To(Succeed())

		id := patient.Id.Hex()
		var count int
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM patient_summaries WHERE patient_id = $1", id).Scan(&count)).To(Succeed())
		Expect(count).To(Equal(2))
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM patient_summary_periods WHERE patient_id = $1", id).Scan(&count)).To(Succeed())
		Expect(count).To(Equal(3))

		// CGM metric columns are populated and typed
		var timeInTarget float64
		var totalRecords int
		var stored time.Time
		row := conn.QueryRow(ctx, `
			SELECT p.time_in_target_percent, p.total_records, s.dates_last_data
			FROM patient_summary_periods p
			JOIN patient_summaries s ON s.patient_id = p.patient_id AND s.summary_type = p.summary_type
			WHERE p.patient_id = $1 AND p.summary_type = 'cgm' AND p.period = '14d'`, id)
		Expect(row.Scan(&timeInTarget, &totalRecords, &stored)).To(Succeed())
		Expect(timeInTarget).To(BeNumerically("~", 0.72, 1e-9))
		Expect(totalRecords).To(Equal(1234))
		Expect(stored.UTC()).To(BeTemporally("==", lastData))

		// CGM-only metrics stay NULL on BGM rows
		var hours *int
		Expect(conn.QueryRow(ctx,
			"SELECT hours_with_data FROM patient_summary_periods WHERE patient_id = $1 AND summary_type = 'bgm' AND period = '14d'",
			id).Scan(&hours)).To(Succeed())
		Expect(hours).To(BeNil())

		// A snapshot without a summary removes all rows
		patient.Summary = nil
		Expect(writer.UpsertPatient(ctx, patient)).To(Succeed())
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM patient_summaries WHERE patient_id = $1", id).Scan(&count)).To(Succeed())
		Expect(count).To(Equal(0))
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM patient_summary_periods WHERE patient_id = $1", id).Scan(&count)).To(Succeed())
		Expect(count).To(Equal(0))
	})

	It("deletes summaries by summary id keeping the other type", func() {
		writer, conn, ctx := newTestWriter()
		patient := randomPatient(primitive.NewObjectID())
		cgmId := primitive.NewObjectID().Hex()
		bgmId := primitive.NewObjectID().Hex()
		patient.Summary = newSummary(cgmId, bgmId, time.Now().UTC())
		Expect(writer.UpsertPatient(ctx, patient)).To(Succeed())

		Expect(writer.DeleteSummariesBySummaryId(ctx, cgmId)).To(Succeed())

		id := patient.Id.Hex()
		var count int
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM patient_summaries WHERE patient_id = $1 AND summary_type = 'cgm'", id).Scan(&count)).To(Succeed())
		Expect(count).To(Equal(0))
		// Period rows cascade with their summary
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM patient_summary_periods WHERE patient_id = $1 AND summary_type = 'cgm'", id).Scan(&count)).To(Succeed())
		Expect(count).To(Equal(0))
		Expect(conn.QueryRow(ctx, "SELECT COUNT(*) FROM patient_summaries WHERE patient_id = $1 AND summary_type = 'bgm'", id).Scan(&count)).To(Succeed())
		Expect(count).To(Equal(1))
	})
})
