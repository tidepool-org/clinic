package merge_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/merge"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
)

const (
	expectedClusters = 50
	inClusterLikelyDuplicateAccountsCount = 2
	inClusterNameOnlyMatchAccountsCount   = 3
	inClusterMRNOnlyMatchAccountsCount    = 4
)

var _ = Describe("Patient Cluster Reporter", func() {
	var source clinics.Clinic
	var sourcePatients []patients.Patient
	var clusters merge.PatientClusters

	BeforeEach(func() {
		source = *clinicsTest.RandomClinic()
		sourcePatients = make([]patients.Patient, patientCount)
		for i := 0; i < patientCount; i++ {
			sourcePatient := patientsTest.RandomPatient()
			sourcePatient.ClinicId = source.Id
			sourcePatients[i] = sourcePatient
		}

		for i := 0; i < expectedClusters; i++ {
			last := sourcePatients[i]
			for j := 0; j < inClusterLikelyDuplicateAccountsCount; j++ {
				last = likelyDuplicatePatientAccount(source.Id, last)
				sourcePatients = append(sourcePatients, last)
			}
			for j := 0; j < inClusterNameOnlyMatchAccountsCount; j++ {
				last = nameOnlyMatchPatientAccount(source.Id, last)
				sourcePatients = append(sourcePatients, last)
			}
			for j := 0; j < inClusterMRNOnlyMatchAccountsCount; j++ {
				last = mrnOnlyMatchPatientAccount(source.Id, last)
				sourcePatients = append(sourcePatients, last)
			}
		}

		reporter := merge.NewPatientClusterReporter(sourcePatients)
		Expect(reporter).ToNot(BeNil())

		var err error
		clusters, err = reporter.GetPatientClusters()
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Clusters", func() {
		It("have the expected number of conflicts", func() {
			Expect(clusters).To(HaveLen(expectedClusters))
		})

		It("have the expected number of duplicates withing the cluster", func() {
			expectedClusterSize := 1 + inClusterLikelyDuplicateAccountsCount + inClusterMRNOnlyMatchAccountsCount + inClusterNameOnlyMatchAccountsCount
			for _, cluster := range clusters {
				Expect(cluster.Patients).To(HaveLen(expectedClusterSize))
			}
		})
	})
})
