package test

import (
	"fmt"
	"math/rand/v2"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/clinicians"
	cliniciansTest "github.com/tidepool-org/clinic/clinicians/test"
	"github.com/tidepool-org/clinic/clinics"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/sites"
	sitesTest "github.com/tidepool-org/clinic/sites/test"
	"github.com/tidepool-org/clinic/test"
)

type Data struct {
	Source         clinics.Clinic
	SourceAdmin    clinicians.Clinician
	SourcePatients []patients.Patient

	Target                       clinics.Clinic
	TargetAdmin                  clinicians.Clinician
	TargetPatients               []patients.Patient
	TargetPatientsWithDuplicates map[string]patients.Patient
}

type Params struct {
	UniquePatientCount           int
	DuplicateAccountsCount       int
	LikelyDuplicateAccountsCount int
	NameOnlyMatchAccountsCount   int
	MrnOnlyMatchAccountsCount    int
}

func RandomData(p Params) Data {
	duplicateCount := p.DuplicateAccountsCount + p.LikelyDuplicateAccountsCount + p.NameOnlyMatchAccountsCount + p.MrnOnlyMatchAccountsCount
	totalCount := 2*p.UniquePatientCount + duplicateCount

	unique := generateUniquePatients(totalCount)

	source := *clinicsTest.RandomClinic()
	sourceAdmin := cliniciansTest.RandomClinician()
	sourceAdmin.ClinicId = source.Id

	target := *clinicsTest.RandomClinic()
	targetAdmin := cliniciansTest.RandomClinician()
	targetAdmin.ClinicId = target.Id

	targetPatientsWithDuplicates := make(map[string]patients.Patient)

	srcDupSite := sitesTest.Random()
	source.Sites = append(source.Sites, srcDupSite)
	tgtDupSite := sitesTest.Random()
	tgtDupSite.Name = srcDupSite.Name
	target.Sites = append(target.Sites, tgtDupSite)

	var sourcePatients, targetPatients []patients.Patient

	unique, sourcePatients = removeTailElements(unique, p.UniquePatientCount)
	for i := range sourcePatients {
		sourcePatients[i].ClinicId = source.Id
		sourcePatients[i].Tags = randomTagIds(len(source.PatientTags)-1, source.PatientTags)
		sourcePatients[i].Sites = &[]sites.Site{source.Sites[rand.IntN(len(source.Sites))]}
	}

	unique, targetPatients = removeTailElements(unique, p.UniquePatientCount)
	for i := range targetPatients {
		targetPatients[i].ClinicId = target.Id
		targetPatients[i].Tags = randomTagIds(len(target.PatientTags)-1, target.PatientTags)
		targetPatients[i].Sites = &[]sites.Site{target.Sites[rand.IntN(len(target.Sites))]}
	}

	i := 0
	for j := 0; j < p.DuplicateAccountsCount; j++ {
		sourcePatient := sourcePatients[i]
		i++

		var targetPatient patients.Patient
		unique, targetPatient = removeTailElement(unique)
		targetPatient.ClinicId = target.Id
		targetPatient.Tags = randomTagIds(len(target.PatientTags)-1, target.PatientTags)

		// Modify the unique patient to make it match the "Duplicate Account" criteria,
		// and append it to the final list of patients
		makeDuplicatePatientAccount(sourcePatient, &targetPatient)

		if j == 0 { // ensure that at least one merging patient has the duplicate site
			*sourcePatients[i].Sites = append(*sourcePatients[i].Sites, srcDupSite)
			*targetPatient.Sites = append(*targetPatient.Sites, tgtDupSite)
		} else if j == 1 { // ensure that at least one merging patient has nil sites
			targetPatient.Sites = nil
			sourcePatient.Sites = nil
		}
		targetPatients = append(targetPatients, targetPatient)
		targetPatientsWithDuplicates[*sourcePatient.UserId] = targetPatient
	}

	for j := 0; j < p.LikelyDuplicateAccountsCount; j++ {
		sourcePatient := sourcePatients[i]
		i++

		var targetPatient patients.Patient
		unique, targetPatient = removeTailElement(unique)
		targetPatient.ClinicId = target.Id

		// Modify the unique patient to make it match the "Likely Duplicate" criteria,
		// and append it to the final list of patients for clustering
		makeLikelyDuplicatePatientAccount(sourcePatient, &targetPatient)
		targetPatients = append(targetPatients, targetPatient)
	}

	for j := 0; j < p.NameOnlyMatchAccountsCount; j++ {
		sourcePatient := sourcePatients[i]
		i++

		var targetPatient patients.Patient
		unique, targetPatient = removeTailElement(unique)
		targetPatient.ClinicId = target.Id

		// Modify the unique patient to make it match the "Name Only" criteria,
		// and append it to the final list of patients for clustering
		makeNameOnlyMatchPatientAccount(sourcePatient, &targetPatient)

		targetPatients = append(targetPatients, targetPatient)
	}

	for j := 0; j < p.MrnOnlyMatchAccountsCount; j++ {
		sourcePatient := sourcePatients[i]
		i++

		var targetPatient patients.Patient
		unique, targetPatient = removeTailElement(unique)
		targetPatient.ClinicId = target.Id

		// Modify the unique patient to make it match the "MRN Only" criteria,
		// and append it to the final list of patients for clustering
		makeMRNOnlyMatchPatientAccount(sourcePatient, &targetPatient)

		targetPatients = append(targetPatients, targetPatient)
	}

	return Data{
		Source:                       source,
		SourceAdmin:                  *sourceAdmin,
		SourcePatients:               sourcePatients,
		Target:                       target,
		TargetAdmin:                  *targetAdmin,
		TargetPatients:               targetPatients,
		TargetPatientsWithDuplicates: targetPatientsWithDuplicates,
	}
}

type ClusterData struct {
	Clinic   clinics.Clinic
	Patients []patients.Patient
}

type ClusterParams struct {
	ClusterCount                          int
	InClusterLikelyDuplicateAccountsCount int
	InClusterNameOnlyMatchAccountsCount   int
	InClusterMRNOnlyMatchAccountsCount    int
}

func RandomDataForClustering(c ClusterParams) ClusterData {
	clinic := *clinicsTest.RandomClinic()
	clusterSize := c.InClusterNameOnlyMatchAccountsCount + c.InClusterMRNOnlyMatchAccountsCount + c.InClusterLikelyDuplicateAccountsCount
	uniqueCount := c.ClusterCount + c.ClusterCount*(clusterSize)

	unique := generateUniquePatients(uniqueCount)
	for _, patient := range unique {
		patient.ClinicId = clinic.Id
	}

	var patientsList []patients.Patient
	unique, patientsList = removeTailElements(unique, c.ClusterCount)

	for i := 0; i < c.ClusterCount; i++ {
		patient := patientsList[i]
		for j := 0; j < c.InClusterLikelyDuplicateAccountsCount; j++ {
			var duplicate patients.Patient
			unique, duplicate = removeTailElement(unique)

			// Modify the unique patient to make it match the "Likely Duplicate" criteria
			// and append it to the final list of patients for clustering
			makeLikelyDuplicatePatientAccount(patient, &duplicate)
			patientsList = append(patientsList, duplicate)
		}
		for j := 0; j < c.InClusterNameOnlyMatchAccountsCount; j++ {
			var duplicate patients.Patient
			unique, duplicate = removeTailElement(unique)

			// Modify the unique patient to make it match the "Name Only" criteria
			// and append it to the final list of patients for clustering
			makeNameOnlyMatchPatientAccount(patient, &duplicate)
			patientsList = append(patientsList, duplicate)
		}
		for j := 0; j < c.InClusterMRNOnlyMatchAccountsCount; j++ {
			var duplicate patients.Patient
			unique, duplicate = removeTailElement(unique)

			// Modify the unique patient to make it match the "MRN Only" criteria
			// and append it to the final list of patients for clustering
			makeMRNOnlyMatchPatientAccount(patient, &duplicate)
			patientsList = append(patientsList, duplicate)
		}
	}

	return ClusterData{
		Clinic:   clinic,
		Patients: patientsList,
	}
}

func makeDuplicatePatientAccount(source patients.Patient, target *patients.Patient) {
	target.UserId = cloneVal(source.UserId)
}

func makeMRNOnlyMatchPatientAccount(source patients.Patient, target *patients.Patient) {
	target.Mrn = cloneVal(source.Mrn)
}

func makeNameOnlyMatchPatientAccount(source patients.Patient, target *patients.Patient) {
	target.FullName = cloneVal(source.FullName)
}

func makeLikelyDuplicatePatientAccount(source patients.Patient, target *patients.Patient) {
	r := test.Rand.Intn(3)
	if r == 0 {
		target.FullName = cloneVal(source.FullName)
		target.BirthDate = cloneVal(source.BirthDate)
	} else if r == 1 {
		target.FullName = cloneVal(source.FullName)
		target.Mrn = cloneVal(source.Mrn)
	} else if r == 2 {
		target.BirthDate = cloneVal(source.BirthDate)
		target.Mrn = cloneVal(source.Mrn)
	}
}

func randomTagIds(count int, tags []clinics.PatientTag) *[]primitive.ObjectID {
	if count > len(tags) {
		count = len(tags)
	}
	test.Rand.Shuffle(len(tags), func(i, j int) {
		tags[i], tags[j] = tags[j], tags[i]
	})
	result := make([]primitive.ObjectID, count)
	for i := range result {
		result[i] = *tags[i].Id
	}
	return &result
}

func generateUnique(generate func() string, count int) []string {
	unique := mapset.NewSet[string]()
	for i := 0; i < count; {
		if unique.Add(generate()) {
			i++
		}
	}
	return unique.ToSlice()
}

func generateUniquePatients(count int) []patients.Patient {
	uniqueBirthDates := generateUnique(func() string {
		return test.Faker.Time().ISO8601(time.Now())[:10]
	}, count)
	uniqueMRNs := generateUnique(test.Faker.UUID().V4, count)
	uniqueNames := generateUnique(test.Faker.Person().Name, count)
	uniqueUserIDs := generateUnique(test.Faker.UUID().V4, count)

	result := make([]patients.Patient, count)
	for i := range result {
		result[i] = patientsTest.RandomPatient()
		result[i].BirthDate = &uniqueBirthDates[i]
		result[i].FullName = &uniqueNames[i]
		result[i].Mrn = &uniqueMRNs[i]
		result[i].UserId = &uniqueUserIDs[i]
	}
	return result
}

func removeTailElement(pts []patients.Patient) ([]patients.Patient, patients.Patient) {
	head, tail := removeTailElements(pts, 1)
	return head, tail[0]
}

func removeTailElements(pts []patients.Patient, count int) ([]patients.Patient, []patients.Patient) {
	if count > len(pts) {
		panic(fmt.Sprintf("cannot remove %d elements from a list with length %d", count, len(pts)))
	}

	tail := append(make([]patients.Patient, 0, count), pts[len(pts)-count:]...)
	head := append(make([]patients.Patient, 0, len(pts)-count), pts[:len(pts)-count]...)
	return head, tail
}

func cloneVal[T *S, S any](p T) T {
	val := *p
	return &val
}
