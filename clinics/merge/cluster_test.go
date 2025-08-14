package merge_test

import (
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/clinics/merge"
	mergeTest "github.com/tidepool-org/clinic/clinics/merge/test"
)

const (
	expectedClusters                      = 50
	inClusterLikelyDuplicateAccountsCount = 2
	inClusterNameOnlyMatchAccountsCount   = 3
	inClusterMRNOnlyMatchAccountsCount    = 4
)

var _ = Describe("Patient Cluster Reporter", func() {
	var clusters merge.PatientClusters

	BeforeEach(func() {
		data := mergeTest.RandomDataForClustering(mergeTest.ClusterParams{
			ClusterCount:                          expectedClusters,
			InClusterLikelyDuplicateAccountsCount: inClusterLikelyDuplicateAccountsCount,
			InClusterNameOnlyMatchAccountsCount:   inClusterNameOnlyMatchAccountsCount,
			InClusterMRNOnlyMatchAccountsCount:    inClusterMRNOnlyMatchAccountsCount,
		})

		reporter := merge.NewPatientClusterReporter(data.Patients)
		Expect(reporter).ToNot(BeNil())

		var err error
		clusters, err = reporter.GetPatientClusters()
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Clusters", func() {
		It("have the expected number of conflicts", func() {
			Expect(clusters).To(HaveLen(expectedClusters))
		})

		It("have the expected number of duplicates within the cluster", func() {
			expectedClusterSize := 1 + inClusterLikelyDuplicateAccountsCount + inClusterMRNOnlyMatchAccountsCount + inClusterNameOnlyMatchAccountsCount
			for i, cluster := range clusters {
				if len(cluster.Patients) != expectedClusterSize {
					res, _ := json.Marshal(cluster.Patients)
					fmt.Println(i)
					fmt.Println(string(res))
				}
				Expect(cluster.Patients).To(HaveLen(expectedClusterSize))
			}
		})
	})
})
