package merge_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tealeg/xlsx/v3"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/tidepool-org/clinic/clinics/merge"
	mergeTest "github.com/tidepool-org/clinic/clinics/merge/test"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

var _ = Describe("Duplicates Report", Ordered, func() {
	var t *ClinicMergeTest
	var plan merge.DuplicatesReportPlan
	var report *xlsx.File

	var clusters merge.PatientClusters
	var data mergeTest.ClusterData

	BeforeAll(func() {
		t = NewClinicMergeTest()
		t.Init(mergeTest.Params{UniquePatientCount: 1})

		data = mergeTest.RandomDataForClustering(mergeTest.ClusterParams{
			ClusterCount:                          3,
			InClusterLikelyDuplicateAccountsCount: 2,
			InClusterNameOnlyMatchAccountsCount:   1,
			InClusterMRNOnlyMatchAccountsCount:    1,
		})
		ctx := context.Background()
		_, err := t.db.Collection("clinics").InsertOne(ctx, data.Clinic)
		Expect(err).ToNot(HaveOccurred())
		documents := make([]any, 0, len(data.Patients))
		for i := range data.Patients {
			data.Patients[i].ClinicId = data.Clinic.Id
			documents = append(documents, data.Patients[i])
		}
		_, err = t.db.Collection("patients").InsertMany(ctx, documents)
		Expect(err).ToNot(HaveOccurred())

		clusters, err = merge.NewPatientClusterReporter(data.Patients).Plan(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(clusters).To(HaveLen(3))

		plan, err = merge.PlanDuplicatesReport(ctx, t.clinicsService, t.patientsService, data.Clinic.Id.Hex())
		Expect(err).ToNot(HaveOccurred())
		report, err = merge.NewDuplicatesReport(plan).Generate()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterAll(func() {
		t.app.RequireStop()
		_, err := dbTest.GetTestDatabase().Collection("patients").DeleteMany(context.Background(), bson.M{})
		Expect(err).To(Succeed())
	})

	It("has the Summary, Patients and Duplicates sheets in order", func() {
		names := make([]string, 0, len(report.Sheets))
		for _, sh := range report.Sheets {
			names = append(names, sh.Name)
		}
		Expect(names).To(Equal([]string{merge.ReportSheetNameSummary, merge.ReportSheetPatients, merge.ReportSheetDuplicates}))
	})

	It("lists every patient of the workspace", func() {
		Expect(plan.Patients).To(HaveLen(len(data.Patients)))
	})

	It("lists every cluster the duplicate matcher finds", func() {
		Expect(plan.PatientClusters).To(HaveLen(len(clusters)))

		groups := 0
		Expect(report.Sheet[merge.ReportSheetDuplicates].ForEachRow(func(r *xlsx.Row) error {
			if value := r.GetCell(0).Value; strings.HasPrefix(value, "Review ") && value != "Review Possible Duplicates" {
				groups++
			}
			return nil
		})).To(Succeed())
		Expect(groups).To(Equal(len(plan.PatientClusters)))
	})
})
