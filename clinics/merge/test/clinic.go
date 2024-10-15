package test

import (
	"github.com/tidepool-org/clinic/clinicians"
	cliniciansTest "github.com/tidepool-org/clinic/clinicians/test"
	"github.com/tidepool-org/clinic/clinics"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math/rand"
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
	PatientCount                 int
	DuplicateAccountsCount       int
	LikelyDuplicateAccountsCount int
	NameOnlyMatchAccountsCount   int
	MrnOnlyMatchAccountsCount    int
}

func RandomData(p Params) Data {
	source := *clinicsTest.RandomClinic()
	sourceAdmin := cliniciansTest.RandomClinician()
	sourceAdmin.ClinicId = source.Id
	sourcePatients := make([]patients.Patient, p.PatientCount)

	target := *clinicsTest.RandomClinic()
	targetAdmin := cliniciansTest.RandomClinician()
	targetAdmin.ClinicId = target.Id
	targetPatients := make([]patients.Patient, p.PatientCount)
	targetPatientsWithDuplicates := make(map[string]patients.Patient)

	for i := 0; i < p.PatientCount; i++ {
		sourcePatient := patientsTest.RandomPatient()
		sourcePatient.ClinicId = source.Id
		sourcePatient.Tags = randomTagIds(len(source.PatientTags)-1, source.PatientTags)
		sourcePatients[i] = sourcePatient

		targetPatient := patientsTest.RandomPatient()
		targetPatient.ClinicId = target.Id
		targetPatient.Tags = randomTagIds(len(target.PatientTags)-1, target.PatientTags)
		targetPatients[i] = targetPatient
	}

	i := 0
	for j := 0; j < p.DuplicateAccountsCount; j++ {
		targetPatient := duplicatePatientAccount(target.Id, sourcePatients[i])
		targetPatient.Tags = randomTagIds(len(target.PatientTags)-1, target.PatientTags)
		targetPatients = append(targetPatients, targetPatient)
		targetPatientsWithDuplicates[*sourcePatients[i].UserId] = targetPatient
		i++
	}
	for j := 0; j < p.LikelyDuplicateAccountsCount; j++ {
		targetPatient := likelyDuplicatePatientAccount(target.Id, sourcePatients[i])
		targetPatients = append(targetPatients, targetPatient)
		i++
	}
	for j := 0; j < p.NameOnlyMatchAccountsCount; j++ {
		targetPatient := nameOnlyMatchPatientAccount(target.Id, sourcePatients[i])
		targetPatients = append(targetPatients, targetPatient)
		i++
	}
	for j := 0; j < p.MrnOnlyMatchAccountsCount; j++ {
		targetPatient := mrnOnlyMatchPatientAccount(target.Id, sourcePatients[i])
		targetPatients = append(targetPatients, targetPatient)
		i++
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
	PatientCount                          int
	ClusterCount                          int
	InClusterLikelyDuplicateAccountsCount int
	InClusterNameOnlyMatchAccountsCount   int
	InClusterMRNOnlyMatchAccountsCount    int
}

func RandomDataForClustering(c ClusterParams) ClusterData {
	clinic := *clinicsTest.RandomClinic()
	patientsList := make([]patients.Patient, c.PatientCount)
	for i := 0; i < c.PatientCount; i++ {
		patient := patientsTest.RandomPatient()
		patient.ClinicId = clinic.Id
		patientsList[i] = patient
	}

	for i := 0; i < c.ClusterCount; i++ {
		last := patientsList[i]
		for j := 0; j < c.InClusterLikelyDuplicateAccountsCount; j++ {
			last = likelyDuplicatePatientAccount(clinic.Id, last)
			patientsList = append(patientsList, last)
		}
		for j := 0; j < c.InClusterNameOnlyMatchAccountsCount; j++ {
			last = nameOnlyMatchPatientAccount(clinic.Id, last)
			patientsList = append(patientsList, last)
		}
		for j := 0; j < c.InClusterMRNOnlyMatchAccountsCount; j++ {
			last = mrnOnlyMatchPatientAccount(clinic.Id, last)
			patientsList = append(patientsList, last)
		}
	}

	return ClusterData{
		Clinic:   clinic,
		Patients: patientsList,
	}
}

func duplicatePatientAccount(clinicId *primitive.ObjectID, patient patients.Patient) patients.Patient {
	duplicate := patientsTest.RandomPatient()
	duplicate.ClinicId = clinicId
	duplicate.UserId = patient.UserId
	return duplicate
}

func mrnOnlyMatchPatientAccount(clinicId *primitive.ObjectID, patient patients.Patient) patients.Patient {
	duplicate := patientsTest.RandomPatient()
	duplicate.ClinicId = clinicId
	duplicate.Mrn = patient.Mrn
	return duplicate
}

func nameOnlyMatchPatientAccount(clinicId *primitive.ObjectID, patient patients.Patient) patients.Patient {
	duplicate := patientsTest.RandomPatient()
	duplicate.ClinicId = clinicId
	duplicate.FullName = patient.FullName
	return duplicate
}

func likelyDuplicatePatientAccount(clinicId *primitive.ObjectID, patient patients.Patient) patients.Patient {
	duplicate := patientsTest.RandomPatient()
	duplicate.ClinicId = clinicId

	r := rand.Intn(3)
	if r == 0 {
		duplicate.FullName = patient.FullName
		duplicate.BirthDate = patient.BirthDate
	} else if r == 1 {
		duplicate.FullName = patient.FullName
		duplicate.Mrn = patient.Mrn
	} else if r == 2 {
		duplicate.BirthDate = patient.BirthDate
		duplicate.Mrn = patient.Mrn
	}

	return duplicate
}

func randomTagIds(count int, tags []clinics.PatientTag) *[]primitive.ObjectID {
	if count > len(tags) {
		count = len(tags)
	}
	rand.Shuffle(len(tags), func(i, j int) {
		tags[i], tags[j] = tags[j], tags[i]
	})
	result := make([]primitive.ObjectID, count)
	for i := range result {
		result[i] = *tags[i].Id
	}
	return &result
}
