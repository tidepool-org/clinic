package integration_test

import (
	"context"
	"math/rand/v2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/fx/fxtest"

	"github.com/tidepool-org/clinic/clinics"
	clinicsRepository "github.com/tidepool-org/clinic/clinics/repository"
	clinicsService "github.com/tidepool-org/clinic/clinics/service"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/patients"
	patientsRepository "github.com/tidepool-org/clinic/patients/repository"
	patientsService "github.com/tidepool-org/clinic/patients/service"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/sites"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

var _ = Describe("Clinics", func() {
	var clinicsSvc clinics.Service
	var clinic *clinics.Clinic
	var chosenSite sites.Site
	var chosenTag clinics.PatientTag

	var randomSiteFromClinic = func(clinic *clinics.Clinic) sites.Site {
		Expect(len(clinic.Sites) > 0).To(Equal(true), "clinic has no sites")
		return clinic.Sites[rand.IntN(len(clinic.Sites))]
	}

	var randomTagFromClinic = func(clinic *clinics.Clinic) clinics.PatientTag {
		Expect(len(clinic.PatientTags) > 0).To(Equal(true), "clinic has no tags")
		return clinic.PatientTags[rand.IntN(len(clinic.PatientTags))]
	}

	BeforeEach(func() {
		ctx := context.Background()
		logger := testLogger()
		database := dbTest.GetTestDatabase()
		lifecycle := fxtest.NewLifecycle(GinkgoT())

		cfg := &config.Config{ClinicDemoPatientUserId: "demo"}
		patientsRepo, err := patientsRepository.NewRepository(cfg, database, logger,
			lifecycle)
		Expect(err).To(Succeed())

		clinicsRepo, err := clinicsRepository.NewRepository(database, logger, lifecycle)
		Expect(err).To(Succeed())

		clinicsSvc, err = clinicsService.NewService(clinicsRepo, patientsRepo, logger)
		Expect(err).To(Succeed())

		randomClinic := clinicsTest.RandomClinic()
		clinic, err = clinicsRepo.Create(ctx, randomClinic)
		Expect(err).To(Succeed())

		patientsSvc, err := patientsService.NewService(cfg, patientsRepo, clinicsSvc, nil,
			logger, database.Client())
		Expect(err).To(Succeed())

		randomPatient := patientsTest.RandomPatient()
		randomPatient.Permissions.Custodian = nil
		randomPatient.ClinicId = clinic.Id
		Expect(randomPatient.UserId).ToNot(BeNil())
		newPatient, err := patientsSvc.Create(ctx, randomPatient)
		Expect(err).To(Succeed())

		chosenSite = randomSiteFromClinic(clinic)
		chosenTag = randomTagFromClinic(clinic)

		patientUpdate := patients.PatientUpdate{}
		patientUpdate.ClinicId = clinic.Id.Hex()
		patientUpdate.UserId = *newPatient.UserId
		newSites := []sites.Site{chosenSite}
		newPatient.Sites = &newSites
		newTagIDs := []primitive.ObjectID{*chosenTag.Id}
		newPatient.Tags = &newTagIDs
		patientUpdate.Patient = *newPatient
		_, err = patientsSvc.Update(ctx, patientUpdate)
		Expect(err).To(Succeed())
	})

	Context("patient counts", func() {
		Describe("ListSites", func() {
			It("returns patients per site", func() {
				ctx := context.Background()

				sitesList, err := clinicsSvc.ListSites(ctx, clinic.Id.Hex())
				Expect(err).To(Succeed())
				Expect(len(sitesList)).To(Equal(2))
				var theSite *sites.Site
				for _, site := range sitesList {
					if site.Id == chosenSite.Id {
						theSite = &site
					} else {
						Expect(site.Patients).To(Equal(0))
					}
				}
				Expect(theSite).ToNot(Equal(nil))
				Expect(theSite.Patients).To(Equal(1))
			})

		})

		Describe("ListPatientTags", func() {
			It("returns patients per tag", func() {
				ctx := context.Background()

				patientTags, err := clinicsSvc.ListPatientTags(ctx, clinic.Id.Hex())
				Expect(err).To(Succeed())
				Expect(len(patientTags)).To(Equal(3))
				var theTag *clinics.PatientTag
				for _, tag := range patientTags {
					if *tag.Id == *chosenTag.Id {
						theTag = &tag
					} else {
						Expect(tag.Patients).To(Equal(0))
					}
				}
				Expect(theTag).ToNot(BeNil())
				Expect(theTag.Patients).To(Equal(1))
			})
		})
	})
})
