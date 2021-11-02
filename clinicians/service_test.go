package clinicians_test

import (
	"context"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinicians"
	cliniciansTest "github.com/tidepool-org/clinic/clinicians/test"
	"github.com/tidepool-org/clinic/clinics"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/logger"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/store"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"sync"
)

var _ = Describe("Clinicians Service", func() {
	var cliniciansService clinicians.Service
	var clinicsService clinics.Service
	var userService *patientsTest.MockUserService
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
					clinics.NewRepository,
					clinicians.NewRepository,
					clinicians.NewService,
				),
				fx.Invoke(func(cliniciansSvc clinicians.Service, clinicsSvc clinics.Service, userSvc patients.UserService) {
					cliniciansService = cliniciansSvc
					clinicsService = clinicsSvc
					userService = userSvc.(*patientsTest.MockUserService)
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
			clinic, err = clinicsService.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			adminId := (*clinic.Admins)[0]
			clinician := cliniciansTest.RandomClinician()
			clinician.UserId = &adminId
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			_, err = cliniciansService.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Creates a clinician in the repository", func() {
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = clinic.Id

			created, err := cliniciansService.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).ToNot(BeNil())
			Expect(created.Id).ToNot(BeNil())
			Expect(created.UserId).ToNot(BeNil())

			result, err := cliniciansService.Get(context.Background(), created.ClinicId.Hex(), *created.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Id).ToNot(BeNil())
			Expect(result.Id.Hex()).To(Equal(created.Id.Hex()))
		})

		It("Adds clinic admins to the clinic", func() {
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			_, err := cliniciansService.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())

			clinic, err = clinicsService.Get(context.Background(), clinic.Id.Hex())
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
			clinic, err = clinicsService.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			adminId := (*clinic.Admins)[0]
			clinician = cliniciansTest.RandomClinician()
			clinician.UserId = &adminId
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			_, err = cliniciansService.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Prevents orphaning a clinic", func() {
			err := cliniciansService.Delete(context.Background(), clinician.ClinicId.Hex(), *clinician.UserId)
			Expect(err).To(MatchError("constraint violation: the clinic must have at least one admin"))
		})

		It("Works when there are multiple admins", func() {
			second := cliniciansTest.RandomClinician()
			second.ClinicId = clinic.Id
			second.Roles = []string{"CLINIC_ADMIN"}

			created, err := cliniciansService.Create(context.Background(), second)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).ToNot(BeNil())
			Expect(created.Id).ToNot(BeNil())
			Expect(created.UserId).ToNot(BeNil())

			err = cliniciansService.Delete(context.Background(), clinician.ClinicId.Hex(), *clinician.UserId)
			Expect(err).ToNot(HaveOccurred())

			clinic, err = clinicsService.Get(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic.Admins).ToNot(BeNil())
			Expect(*clinic.Admins).ToNot(ContainElement(*clinician.UserId))
		})

		It("Adds clinic admins to the clinic", func() {
			clinician := cliniciansTest.RandomClinician()
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			_, err := cliniciansService.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())

			clinic, err = clinicsService.Get(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic.Admins).ToNot(BeNil())
			Expect(*clinic.Admins).To(ContainElement(*clinician.UserId))
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
			clinic, err = clinicsService.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			for i := range members {
				members[i] = cliniciansTest.RandomClinician()
				members[i].ClinicId = clinic.Id
				members[i].Roles = []string{"CLINIC_MEMBER"}
				_, err = cliniciansService.Create(context.Background(), members[i])
				Expect(err).ToNot(HaveOccurred())
			}

			for i := range admins {
				admins[i] = cliniciansTest.RandomClinician()
				admins[i].ClinicId = clinic.Id
				admins[i].Roles = []string{"CLINIC_ADMIN"}
				_, err = cliniciansService.Create(context.Background(), admins[i])
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			for _, clinician := range admins {
				err := cliniciansService.Delete(context.Background(), clinic.Id.Hex(), *clinician.UserId)
				Expect(err).ToNot(HaveOccurred())
			}
			for _, clinician := range members {
				err := cliniciansService.Delete(context.Background(), clinic.Id.Hex(), *clinician.UserId)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("Applies role filter correctly", func() {
			role := cliniciansTest.Faker.RandomStringElement([]string{"CLINIC_ADMIN", "CLINIC_MEMBER"})
			filter := clinicians.Filter{
				Role: &role,
			}
			pagination := store.Pagination{}

			results, err := cliniciansService.List(context.Background(), &filter, pagination)
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
			clinic, err = clinicsService.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			adminId := (*clinic.Admins)[0]
			clinician = cliniciansTest.RandomClinician()
			clinician.UserId = &adminId
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			clinician, err = cliniciansService.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Updates an existing clinician", func() {
			updatedName := cliniciansTest.Faker.Person().Name()

			result, err := cliniciansService.Get(context.Background(), clinician.ClinicId.Hex(), *clinician.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			result.Name = &updatedName
			clinicianUpdate := &clinicians.ClinicianUpdate{
				UpdatedBy:   cliniciansTest.Faker.UUID().V4(),
				ClinicId:    clinician.ClinicId.Hex(),
				ClinicianId: *clinician.UserId,
				Clinician:   *result,
			}
			updated, err := cliniciansService.Update(context.Background(), clinicianUpdate)
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).ToNot(BeNil())
			Expect(updated.Name).ToNot(BeNil())
			Expect(*updated.Name).To(Equal(updatedName))
		})

		It("Adds an admin to the clinic", func() {
			newClinician := cliniciansTest.RandomClinician()
			newClinician.ClinicId = clinic.Id
			newClinician.Roles = []string{"CLINIC_MEMBER"}

			newClinician, err := cliniciansService.Create(context.Background(), newClinician)
			Expect(err).ToNot(HaveOccurred())

			newClinician.Id = nil // Id is immutable
			newClinician.Roles = []string{"CLINIC_ADMIN"}
			clinicianUpdate := &clinicians.ClinicianUpdate{
				UpdatedBy:   cliniciansTest.Faker.UUID().V4(),
				ClinicId:    clinician.ClinicId.Hex(),
				ClinicianId: *clinician.UserId,
				Clinician:   *clinician,
			}
			newClinician, err = cliniciansService.Update(context.Background(), clinicianUpdate)
			Expect(err).ToNot(HaveOccurred())

			clinic, err = clinicsService.Get(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic.Admins).ToNot(BeNil())
			Expect(*clinic.Admins).To(ContainElement(*newClinician.UserId))
			Expect(*clinic.Admins).To(ContainElement(*clinician.UserId))
		})

		It("Removes an admin of the clinic", func() {
			newClinician := cliniciansTest.RandomClinician()
			newClinician.ClinicId = clinic.Id
			newClinician.Roles = []string{"CLINIC_ADMIN"}

			_, err := cliniciansService.Create(context.Background(), newClinician)
			Expect(err).ToNot(HaveOccurred())

			clinician.Roles = []string{"CLINIC_MEMBER"}
			clinicianUpdate := &clinicians.ClinicianUpdate{
				UpdatedBy:   cliniciansTest.Faker.UUID().V4(),
				ClinicId:    clinician.ClinicId.Hex(),
				ClinicianId: *clinician.UserId,
				Clinician:   *clinician,
			}
			_, err = cliniciansService.Update(context.Background(), clinicianUpdate)
			Expect(err).ToNot(HaveOccurred())

			clinic, err = clinicsService.Get(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic.Admins).ToNot(BeNil())
			Expect(*clinic.Admins).ToNot(ContainElement(*clinician.UserId))
		})

		It("Prevents orphaning a clinic", func() {
			updatedRoles := []string{"CLINIC_MEMBER"}

			result, err := cliniciansService.Get(context.Background(), clinician.ClinicId.Hex(), *clinician.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			result.Roles = updatedRoles
			clinicianUpdate := &clinicians.ClinicianUpdate{
				UpdatedBy:   cliniciansTest.Faker.UUID().V4(),
				ClinicId:    clinician.ClinicId.Hex(),
				ClinicianId: *clinician.UserId,
				Clinician:   *result,
			}
			_, err = cliniciansService.Update(context.Background(), clinicianUpdate)
			Expect(err).To(MatchError("constraint violation: the clinic must have at least one admin"))
		})

		It("Updates the roles history", func() {
			updatedRoles := []string{"CLINIC_ADMIN", "PRESCRIBER"}

			result, err := cliniciansService.Get(context.Background(), clinician.ClinicId.Hex(), *clinician.UserId)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			result.Roles = updatedRoles
			clinicianUpdate := &clinicians.ClinicianUpdate{
				UpdatedBy:   cliniciansTest.Faker.UUID().V4(),
				ClinicId:    clinician.ClinicId.Hex(),
				ClinicianId: *clinician.UserId,
				Clinician:   *result,
			}
			updated, err := cliniciansService.Update(context.Background(), clinicianUpdate)
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).ToNot(BeNil())
			Expect(updated.RolesUpdates).To(HaveLen(1))
			Expect(updated.RolesUpdates[0].UpdatedBy).To(Equal(clinicianUpdate.UpdatedBy))
			Expect(updated.RolesUpdates[0].Roles).To(ConsistOf(updatedRoles))
		})
	})

	Describe("Associate invite", func() {
		var clinic *clinics.Clinic
		var clinician *clinicians.Clinician
		var invite *clinicians.Clinician

		BeforeEach(func() {
			clinic = clinicsTest.RandomClinic()

			var err error
			clinic, err = clinicsService.Create(context.Background(), clinic)
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic).ToNot(BeNil())

			adminId := (*clinic.Admins)[0]
			clinician = cliniciansTest.RandomClinician()
			clinician.UserId = &adminId
			clinician.ClinicId = clinic.Id
			clinician.Roles = []string{"CLINIC_ADMIN"}

			_, err = cliniciansService.Create(context.Background(), clinician)
			Expect(err).ToNot(HaveOccurred())

			invite = cliniciansTest.RandomClinicianInvite()
			invite.ClinicId = clinic.Id
			invite.Roles = []string{"CLINIC_ADMIN"}
			_, err = cliniciansService.Create(context.Background(), invite)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("Adds the user id to the clinic admins attribute of a clinic", func() {
			userId := cliniciansTest.Faker.UUID().V4()
			name := cliniciansTest.Faker.Person().Name()

			association := clinicians.AssociateInvite{
				ClinicId: clinic.Id.Hex(),
				InviteId: *invite.InviteId,
				UserId:   userId,
			}

			userService.EXPECT().
				GetUserProfile(gomock.Any(), gomock.Eq(userId)).
				Return(&patients.Profile{
					FullName: &name,
				}, nil)

			result, err := cliniciansService.AssociateInvite(context.Background(), association)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Name).ToNot(BeNil())
			Expect(*result.Name).To(Equal(name))

			clinic, err = clinicsService.Get(context.Background(), clinic.Id.Hex())
			Expect(err).ToNot(HaveOccurred())
			Expect(clinic.Admins).ToNot(BeNil())
			Expect(*clinic.Admins).To(ContainElement(userId))
		})
	})

})
