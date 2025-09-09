package service_test

import (
	"context"

	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/clinicians"
	cliniciansRepository "github.com/tidepool-org/clinic/clinicians/repository"
	cliniciansService "github.com/tidepool-org/clinic/clinicians/service"
	cliniciansTest "github.com/tidepool-org/clinic/clinicians/test"
	"github.com/tidepool-org/clinic/clinics"
	clinicsRepository "github.com/tidepool-org/clinic/clinics/repository"
	clinicsService "github.com/tidepool-org/clinic/clinics/service"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/logger"
	"github.com/tidepool-org/clinic/patients"
	patientsRepository "github.com/tidepool-org/clinic/patients/repository"
	patientsService "github.com/tidepool-org/clinic/patients/service"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/store"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

var _ = Describe("Clinicians Service", func() {
	var cliniciansSvc clinicians.Service
	var clinicsSvc clinics.Service
	var userSvc *patientsTest.MockUserService
	var patientsSvc patients.Service
	var ctrl *gomock.Controller
	var app *fxtest.App
	beforeOnce := sync.Once{}
	afterOnce := sync.Once{}

	BeforeEach(func() {
		tb := GinkgoT()
		ctrl = gomock.NewController(tb)

		beforeOnce.Do(func() {
			app = fxtest.New(tb,
				fx.Provide(
					zap.NewNop,
					logger.Suggar,
					dbTest.GetTestDatabase,
					func(database *mongo.Database) *mongo.Client {
						return database.Client()
					},
					func() patients.UserService {
						return patientsTest.NewMockUserService(ctrl)
					},
					patientsTest.NewMockUserService,
					config.NewConfig,
					clinicsRepository.NewRepository,
					clinicsService.NewService,
					cliniciansRepository.NewRepository,
					cliniciansService.NewService,
					patientsRepository.NewRepository,
					patientsService.NewService,
					patientsService.NewCustodialService,
				),
				fx.Invoke(func(cliniciansService clinicians.Service, clinicsService clinics.Service, userService patients.UserService, patientsService patients.Service) {
					cliniciansSvc = cliniciansService
					clinicsSvc = clinicsService
					userSvc = userService.(*patientsTest.MockUserService)
					patientsSvc = patientsService
				}),
			)
			app.RequireStart()
		})

	})

	AfterEach(func() {
		afterOnce.Do(func() {
			app.RequireStop()
		})
	})

	Describe("Create clinician", func() {
		var clinic *clinics.Clinic

		BeforeEach(func() {
			clinic = clinicsTest.RandomClinic()

			var err error
			clinic, err = clinicsSvc.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			adminId := (*clinic.Admins)[0]
			clinician := cliniciansTest.RandomClinician()
			clinician.UserId = &adminId
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			_, err = cliniciansSvc.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Creates a clinician in the repository", func() {
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = clinic.Id

			created, err := cliniciansSvc.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).ToNot(BeNil())
			Expect(created.Id).ToNot(BeNil())
			Expect(created.UserId).ToNot(BeNil())

			result, err := cliniciansSvc.Get(context.Background(), created.ClinicId.Hex(), *created.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Id).ToNot(BeNil())
			Expect(result.Id.Hex()).To(Equal(created.Id.Hex()))
		})

		It("Adds clinic admins to the clinic", func() {
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			_, err := cliniciansSvc.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())

			clinic, err = clinicsSvc.Get(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic.Admins).ToNot(BeNil())
			Expect(*clinic.Admins).To(ContainElement(*clinician.UserId))
		})
	})

	Describe("Delete clinician", func() {
		var clinic *clinics.Clinic
		var clinician *clinicians.Clinician

		BeforeEach(func() {
			clinic = clinicsTest.RandomClinic()

			var err error
			clinic, err = clinicsSvc.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			adminId := (*clinic.Admins)[0]
			clinician = cliniciansTest.RandomClinician()
			clinician.UserId = &adminId
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			_, err = cliniciansSvc.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Prevents orphaning a clinic", func() {
			err := cliniciansSvc.Delete(context.Background(), clinician.ClinicId.Hex(), *clinician.UserId, deletions.Metadata{})
			Expect(err).To(MatchError("constraint violation: the clinic must have at least one admin"))

			res, err := cliniciansSvc.Get(context.Background(), clinician.ClinicId.Hex(), *clinician.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.ClinicId).ToNot(BeNil())
			Expect(res.ClinicId.Hex()).To(Equal(clinician.ClinicId.Hex()))
			Expect(res.UserId).To(gstruct.PointTo(Equal(*clinician.UserId)))
		})

		It("Allows orphaning when deleting from all clinics", func() {
			err := cliniciansSvc.DeleteFromAllClinics(context.Background(), *clinician.UserId, deletions.Metadata{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("Works when there are multiple admins", func() {
			second := cliniciansTest.RandomClinician()
			second.ClinicId = clinic.Id
			second.Roles = []string{"CLINIC_ADMIN"}

			created, err := cliniciansSvc.Create(context.Background(), second)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).ToNot(BeNil())
			Expect(created.Id).ToNot(BeNil())
			Expect(created.UserId).ToNot(BeNil())

			err = cliniciansSvc.Delete(context.Background(), clinician.ClinicId.Hex(), *clinician.UserId, deletions.Metadata{})
			Expect(err).ToNot(HaveOccurred())

			clinic, err = clinicsSvc.Get(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic.Admins).ToNot(BeNil())
			Expect(*clinic.Admins).ToNot(ContainElement(*clinician.UserId))
		})

		It("Adds clinic admins to the clinic", func() {
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			_, err := cliniciansSvc.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())

			clinic, err = clinicsSvc.Get(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic.Admins).ToNot(BeNil())
			Expect(*clinic.Admins).To(ContainElement(*clinician.UserId))
		})
	})

	Describe("Delete clinician from all clinics", func() {
		var clinician *clinicians.Clinician
		var clinicsList []*clinics.Clinic

		BeforeEach(func() {
			clinicsList = []*clinics.Clinic{clinicsTest.RandomClinic(), clinicsTest.RandomClinic()}
			clinician = cliniciansTest.RandomClinician()

			for i, clinic := range clinicsList {
				var err error
				clinic.Admins = &[]string{*clinician.UserId}
				clinic, err = clinicsSvc.Create(context.Background(), clinic)
				Expect(err).ToNot(HaveOccurred())
				Expect(clinic).ToNot(BeNil())

				clinicsList[i] = clinic
				clinician.ClinicId = clinic.Id
				clinician.Roles = []string{"CLINIC_ADMIN"}

				_, err = cliniciansSvc.Create(context.Background(), clinician)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("Allows orphaning when deleting from all clinics", func() {
			err := cliniciansSvc.DeleteFromAllClinics(context.Background(), *clinician.UserId, deletions.Metadata{})
			Expect(err).ToNot(HaveOccurred())

			for _, clinic := range clinicsList {
				result, err := cliniciansSvc.Get(context.Background(), clinic.Id.Hex(), *clinician.UserId)
				Expect(err).To(Equal(clinicians.ErrNotFound))
				Expect(result).To(BeNil())
			}
		})

		It("Deletes non-custodial patients of a clinic when clinic is orphaned", func() {
			// Create a patient so we can check later it was deleted from the orphaned clinic
			patient := patientsTest.RandomPatient()
			patient.ClinicId = clinician.ClinicId
			patient.Permissions = &patients.Permissions{
				View: &patients.Permission{},
			}
			_, err := patientsSvc.Create(context.Background(), patient)
			Expect(err).ToNot(HaveOccurred())

			// Delete all clinician records
			err = cliniciansSvc.DeleteFromAllClinics(context.Background(), *clinician.UserId, deletions.Metadata{})
			Expect(err).ToNot(HaveOccurred())

			// Check clinicians records were deleted
			for _, clinic := range clinicsList {
				result, err := cliniciansSvc.Get(context.Background(), clinic.Id.Hex(), *clinician.UserId)
				Expect(err).To(Equal(clinicians.ErrNotFound))
				Expect(result).To(BeNil())
			}

			// Check non-custodial patient was deleted
			_, err = patientsSvc.Get(context.Background(), patient.ClinicId.Hex(), *patient.UserId)
			Expect(err).To(Equal(patients.ErrNotFound))
		})

		It("Does not delete custodial patients of a clinic when clinic is orphaned", func() {
			// Create a patient so we can check later it was not deleted from the orphaned clinic
			patient := patientsTest.RandomPatient()
			patient.ClinicId = clinician.ClinicId
			patient.Permissions = &patients.CustodialAccountPermissions
			_, err := patientsSvc.Create(context.Background(), patient)
			Expect(err).ToNot(HaveOccurred())

			// Delete all clinician records
			err = cliniciansSvc.DeleteFromAllClinics(context.Background(), *clinician.UserId, deletions.Metadata{})
			Expect(err).ToNot(HaveOccurred())

			// Check clinicians records were deleted
			for _, clinic := range clinicsList {
				result, err := cliniciansSvc.Get(context.Background(), clinic.Id.Hex(), *clinician.UserId)
				Expect(err).To(Equal(clinicians.ErrNotFound))
				Expect(result).To(BeNil())
			}

			// Check custodial patient was not deleted
			_, err = patientsSvc.Get(context.Background(), patient.ClinicId.Hex(), *patient.UserId)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("List clinicians", func() {
		const count = 10
		var clinic *clinics.Clinic
		var members []*clinicians.Clinician
		var admins []*clinicians.Clinician

		BeforeEach(func() {
			clinic = clinicsTest.RandomClinic()
			members = make([]*clinicians.Clinician, count)
			admins = make([]*clinicians.Clinician, count)

			var err error
			clinic, err = clinicsSvc.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			for i := range members {
				members[i] = cliniciansTest.RandomClinician()
				members[i].ClinicId = clinic.Id
				members[i].Roles = []string{"CLINIC_MEMBER"}
				_, err = cliniciansSvc.Create(context.Background(), members[i])
				Expect(err).ToNot(HaveOccurred())
			}

			for i := range admins {
				admins[i] = cliniciansTest.RandomClinician()
				admins[i].ClinicId = clinic.Id
				admins[i].Roles = []string{"CLINIC_ADMIN"}
				_, err = cliniciansSvc.Create(context.Background(), admins[i])
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			for _, clinician := range admins {
				err := cliniciansSvc.Delete(context.Background(), clinic.Id.Hex(), *clinician.UserId, deletions.Metadata{})
				Expect(err).ToNot(HaveOccurred())
			}
			for _, clinician := range members {
				err := cliniciansSvc.Delete(context.Background(), clinic.Id.Hex(), *clinician.UserId, deletions.Metadata{})
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("Applies role filter correctly", func() {
			role := test.Faker.RandomStringElement([]string{"CLINIC_ADMIN", "CLINIC_MEMBER"})
			filter := clinicians.Filter{
				Role: &role,
			}
			pagination := store.Pagination{}

			results, err := cliniciansSvc.List(context.Background(), &filter, pagination)
			Expect(err).ToNot(HaveOccurred())
			Expect(results).ToNot(BeEmpty())

			for _, clinician := range results {
				Expect(clinician.Roles).To(ContainElement(role))
			}
		})
	})

	Describe("Update clinician", func() {
		var clinic *clinics.Clinic
		var clinician *clinicians.Clinician

		BeforeEach(func() {
			clinic = clinicsTest.RandomClinic()

			var err error
			clinic, err = clinicsSvc.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			adminId := (*clinic.Admins)[0]
			clinician = cliniciansTest.RandomClinician()
			clinician.UserId = &adminId
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			clinician, err = cliniciansSvc.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Updates an existing clinician", func() {
			updatedName := test.Faker.Person().Name()

			result, err := cliniciansSvc.Get(context.Background(), clinician.ClinicId.Hex(), *clinician.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			result.Name = &updatedName
			clinicianUpdate := &clinicians.ClinicianUpdate{
				UpdatedBy:   test.Faker.UUID().V4(),
				ClinicId:    clinician.ClinicId.Hex(),
				ClinicianId: *clinician.UserId,
				Clinician:   *result,
			}
			updated, err := cliniciansSvc.Update(context.Background(), clinicianUpdate)
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).ToNot(BeNil())
			Expect(updated.Name).ToNot(BeNil())
			Expect(*updated.Name).To(Equal(updatedName))
		})

		It("Adds an admin to the clinic", func() {
			newClinician := cliniciansTest.RandomClinician()
			newClinician.ClinicId = clinic.Id
			newClinician.Roles = []string{"CLINIC_MEMBER"}

			newClinician, err := cliniciansSvc.Create(context.Background(), newClinician)
			Expect(err).ToNot(HaveOccurred())
			Expect(newClinician).ToNot(BeNil())

			clinicianUpdate := &clinicians.ClinicianUpdate{
				UpdatedBy:   test.Faker.UUID().V4(),
				ClinicId:    clinician.ClinicId.Hex(),
				ClinicianId: *clinician.UserId,
				Clinician: clinicians.Clinician{
					Roles: []string{"CLINIC_ADMIN"},
				},
			}
			newClinician, err = cliniciansSvc.Update(context.Background(), clinicianUpdate)
			Expect(err).ToNot(HaveOccurred())

			clinic, err = clinicsSvc.Get(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic.Admins).ToNot(BeNil())
			Expect(*clinic.Admins).To(ContainElement(*newClinician.UserId))
			Expect(*clinic.Admins).To(ContainElement(*clinician.UserId))
		})

		It("Removes an admin of the clinic", func() {
			// Make sure we're not orphaning the clinic
			newClinician := cliniciansTest.RandomClinician()
			newClinician.ClinicId = clinic.Id
			newClinician.Roles = []string{"CLINIC_ADMIN"}
			_, err := cliniciansSvc.Create(context.Background(), newClinician)
			Expect(err).ToNot(HaveOccurred())

			clinicianUpdate := &clinicians.ClinicianUpdate{
				UpdatedBy:   test.Faker.UUID().V4(),
				ClinicId:    clinician.ClinicId.Hex(),
				ClinicianId: *clinician.UserId,
				Clinician: clinicians.Clinician{
					Roles: []string{"CLINIC_MEMBER"},
				},
			}
			_, err = cliniciansSvc.Update(context.Background(), clinicianUpdate)
			Expect(err).ToNot(HaveOccurred())

			clinic, err = clinicsSvc.Get(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic.Admins).ToNot(BeNil())
			Expect(*clinic.Admins).ToNot(ContainElement(*clinician.UserId))
		})

		It("Prevents orphaning a clinic", func() {
			clinicianUpdate := &clinicians.ClinicianUpdate{
				UpdatedBy:   test.Faker.UUID().V4(),
				ClinicId:    clinician.ClinicId.Hex(),
				ClinicianId: *clinician.UserId,
				Clinician: clinicians.Clinician{
					Roles: []string{"CLINIC_MEMBER"},
				},
			}
			_, err := cliniciansSvc.Update(context.Background(), clinicianUpdate)
			Expect(err).To(MatchError("constraint violation: the clinic must have at least one admin"))

			res, err := cliniciansSvc.Get(context.Background(), clinician.ClinicId.Hex(), *clinician.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).ToNot(BeNil())
			Expect(res.ClinicId).ToNot(BeNil())
			Expect(res.ClinicId.Hex()).To(Equal(clinician.ClinicId.Hex()))
			Expect(res.UserId).To(gstruct.PointTo(Equal(*clinician.UserId)))
		})

		It("Updates the roles history", func() {
			roles := []string{"CLINIC_ADMIN", "PRESCRIBER"}
			clinicianUpdate := &clinicians.ClinicianUpdate{
				UpdatedBy:   test.Faker.UUID().V4(),
				ClinicId:    clinician.ClinicId.Hex(),
				ClinicianId: *clinician.UserId,
				Clinician: clinicians.Clinician{
					Roles: roles,
				},
			}
			updated, err := cliniciansSvc.Update(context.Background(), clinicianUpdate)
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).ToNot(BeNil())
			Expect(updated.RolesUpdates).To(HaveLen(1))
			Expect(updated.RolesUpdates[0].UpdatedBy).To(Equal(clinicianUpdate.UpdatedBy))
			Expect(updated.RolesUpdates[0].Roles).To(ConsistOf(roles))
		})
	})

	Describe("Associate invite", func() {
		var clinic *clinics.Clinic
		var clinician *clinicians.Clinician
		var invite *clinicians.Clinician

		BeforeEach(func() {
			clinic = clinicsTest.RandomClinic()

			var err error
			clinic, err = clinicsSvc.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			adminId := (*clinic.Admins)[0]
			clinician = cliniciansTest.RandomClinician()
			clinician.UserId = &adminId
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			_, err = cliniciansSvc.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())

			invite = cliniciansTest.RandomClinicianInvite()
			invite.ClinicId = clinic.Id
			invite.Roles = []string{"CLINIC_ADMIN"}
			_, err = cliniciansSvc.Create(context.Background(), invite)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("Adds the user id to the clinic admins attribute of a clinic", func() {
			userId := test.Faker.UUID().V4()
			name := test.Faker.Person().Name()

			association := clinicians.AssociateInvite{
				ClinicId: clinic.Id.Hex(),
				InviteId: *invite.InviteId,
				UserId:   userId,
			}

			userSvc.EXPECT().
				GetUserProfile(gomock.Any(), gomock.Eq(userId)).
				Return(&patients.Profile{
					FullName: &name,
				}, nil)

			result, err := cliniciansSvc.AssociateInvite(context.Background(), association)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Name).ToNot(BeNil())
			Expect(*result.Name).To(Equal(name))

			clinic, err = clinicsSvc.Get(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic.Admins).ToNot(BeNil())
			Expect(*clinic.Admins).To(ContainElement(userId))
		})
	})

})
