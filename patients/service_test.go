package patients_test

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/tidepool-org/clinic/clinics"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/test"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

func Ptr[T any](value T) *T {
	return &value
}

var _ = Describe("Patients Service", func() {
	var service patients.Service
	var clinicsService *clinicsTest.MockService
	var repo *patientsTest.MockRepository
	var repoCtrl *gomock.Controller
	var clinicsCtrl *gomock.Controller

	BeforeEach(func() {
		repoCtrl = gomock.NewController(GinkgoT())
		clinicsCtrl = gomock.NewController(GinkgoT())

		repo = patientsTest.NewMockRepository(repoCtrl)
		clinicsService = clinicsTest.NewMockService(clinicsCtrl)

		var err error
		service, err = patients.NewService(repo, clinicsService, nil, zap.NewNop().Sugar())
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		repoCtrl.Finish()
		clinicsCtrl.Finish()
	})

	Describe("Create", func() {
		var clinicId primitive.ObjectID
		var randomPatient patients.Patient
		var matchPatientFields types.GomegaMatcher

		BeforeEach(func() {
			clinicId, _ = primitive.ObjectIDFromHex("60d1dc0eac5285751add8f82")
			patientId := primitive.NewObjectID()
			randomPatient = patientsTest.RandomPatient()
			randomPatient.Id = &patientId
			randomPatient.ClinicId = &clinicId
			randomPatient.Permissions = &patients.Permissions{
				Upload: &patients.Permission{},
			}

			matchPatientFields = patientFieldsMatcher(randomPatient)
		})

		When("the clinic requires the mrn to be set", func() {
			BeforeEach(func() {
				clinicsService.
					EXPECT().
					GetMRNSettings(gomock.Any(), gomock.Eq(clinicId.Hex())).
					Return(&clinics.MRNSettings{Required: true}, nil)
			})

			It("creates the patient in the repository when the MRN is set", func() {
				clinicIdString := clinicId.Hex()
				patientCount := &clinics.PatientCount{PatientCount: 10}

				repo.EXPECT().
					Create(gomock.Any(), gomock.Eq(randomPatient)).
					Return(&randomPatient, true, nil)
				repo.EXPECT().
					Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: &clinicIdString, ExcludeDemo: true})).
					Return(patientCount.PatientCount, nil)
				clinicsService.EXPECT().
					UpdatePatientCount(gomock.Any(), gomock.Eq(clinicIdString), gomock.Eq(patientCount)).
					Return(nil)

				createdPatient, created, err := service.Create(context.Background(), randomPatient)
				Expect(err).To(BeNil())
				Expect(createdPatient).ToNot(BeNil())
				Expect(*createdPatient).To(matchPatientFields)
				Expect(created).To(BeTrue())
			})

			It("returns an error when the MRN is not set", func() {
				randomPatient.Mrn = nil

				createdPatient, created, err := service.Create(context.Background(), randomPatient)
				Expect(err).To(MatchError(errors.BadRequest))
				Expect(createdPatient).To(BeNil())
				Expect(created).To(BeFalse())
			})
		})

		When("the clinic requires mrn to be unique", func() {
			BeforeEach(func() {
				clinicsService.
					EXPECT().
					GetMRNSettings(gomock.Any(), gomock.Eq(clinicId.Hex())).
					Return(&clinics.MRNSettings{Unique: true}, nil)
			})

			It("creates the patient in the repository with uniqueness flag set to true", func() {
				create := randomPatient
				clinicIdStr := clinicId.Hex()
				patientCount := &clinics.PatientCount{PatientCount: 10}

				// Expect the uniqueness flag to be set to true
				expected := create
				expected.RequireUniqueMrn = true

				repo.EXPECT().
					Create(gomock.Any(), gomock.Eq(expected)).
					Return(&expected, true, nil)
				repo.EXPECT().
					Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: &clinicIdStr, ExcludeDemo: true})).
					Return(patientCount.PatientCount, nil)
				clinicsService.EXPECT().
					UpdatePatientCount(gomock.Any(), gomock.Eq(clinicIdStr), gomock.Eq(patientCount)).
					Return(nil)

				repo.EXPECT().
					List(gomock.Any(), &patients.Filter{ClinicId: &clinicIdStr, Mrn: create.Mrn}, gomock.Any(), gomock.Any()).
					Return(&patients.ListResult{
						Patients:   nil,
						TotalCount: 0,
					}, nil)

				createdPatient, created, err := service.Create(context.Background(), create)
				Expect(err).To(BeNil())
				Expect(createdPatient).ToNot(BeNil())
				Expect(created).To(BeTrue())
			})

			It("returns an error if a patient with the same mrn exists in the repository", func() {
				create := randomPatient
				clinicIdStr := clinicId.Hex()

				// Expect the uniqueness flag to be set to true
				expected := create
				expected.RequireUniqueMrn = true

				existing := patientsTest.RandomPatient()
				existing.Mrn = create.Mrn

				repo.EXPECT().
					List(gomock.Any(), &patients.Filter{ClinicId: &clinicIdStr, Mrn: create.Mrn}, gomock.Any(), gomock.Any()).
					Return(&patients.ListResult{
						Patients:   []*patients.Patient{&existing},
						TotalCount: 1,
					}, nil)

				createdPatient, created, err := service.Create(context.Background(), create)
				Expect(err).To(MatchError("bad request: mrn must be unique"))
				Expect(createdPatient).To(BeNil())
				Expect(created).To(BeFalse())
			})
		})

		When("there there may be a patient count hard limit", func() {
			var now time.Time
			var clinicIdString string
			var patientCount *clinics.PatientCount
			var patientCountSettings *clinics.PatientCountSettings

			BeforeEach(func() {
				now = time.Now()
				clinicIdString = clinicId.Hex()
				patientCount = &clinics.PatientCount{PatientCount: 9}
				patientCountSettings = &clinics.PatientCountSettings{
					HardLimit: &clinics.PatientCountLimit{
						PatientCount: 10,
						StartDate:    Ptr(now.Add(-time.Hour)),
						EndDate:      Ptr(now.Add(time.Hour)),
					},
					SoftLimit: &clinics.PatientCountLimit{
						PatientCount: 1,
						StartDate:    Ptr(now.Add(-time.Hour)),
						EndDate:      Ptr(now.Add(time.Hour)),
					},
				}

				clinicsService.EXPECT().
					GetMRNSettings(gomock.Any(), gomock.Eq(clinicId.Hex())).
					Return(nil, nil)
			})

			It("creates the patient in the repository when the patient is not custodial", func() {
				repo.EXPECT().
					Create(gomock.Any(), gomock.Eq(randomPatient)).
					Return(&randomPatient, true, nil)
				repo.EXPECT().
					Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: &clinicIdString, ExcludeDemo: true})).
					Return(patientCount.PatientCount, nil)
				clinicsService.EXPECT().
					UpdatePatientCount(gomock.Any(), gomock.Eq(clinicIdString), gomock.Eq(patientCount)).
					Return(nil)

				createdPatient, created, err := service.Create(context.Background(), randomPatient)
				Expect(err).ToNot(HaveOccurred())
				Expect(createdPatient).ToNot(BeNil())
				Expect(created).To(BeTrue())
			})

			When("the patient is custodial", func() {
				BeforeEach(func() {
					randomPatient.Permissions.Custodian = &patients.Permission{}
				})

				It("returns an error when GetPatientCountSettings returns an error", func() {
					testErr := fmt.Errorf("test error")

					clinicsService.EXPECT().
						GetPatientCountSettings(gomock.Any(), gomock.Eq(clinicId.Hex())).
						Return(nil, testErr)

					createdPatient, created, err := service.Create(context.Background(), randomPatient)
					Expect(err).To(Equal(testErr))
					Expect(createdPatient).To(BeNil())
					Expect(created).To(BeFalse())
				})

				It("creates the patient in the repository when there are no patient count settings", func() {
					clinicsService.EXPECT().
						GetPatientCountSettings(gomock.Any(), gomock.Eq(clinicId.Hex())).
						Return(nil, nil)
					repo.EXPECT().
						Create(gomock.Any(), gomock.Eq(randomPatient)).
						Return(&randomPatient, true, nil)
					repo.EXPECT().
						Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: &clinicIdString, ExcludeDemo: true})).
						Return(patientCount.PatientCount, nil)
					clinicsService.EXPECT().
						UpdatePatientCount(gomock.Any(), gomock.Eq(clinicIdString), gomock.Eq(patientCount)).
						Return(nil)

					createdPatient, created, err := service.Create(context.Background(), randomPatient)
					Expect(err).ToNot(HaveOccurred())
					Expect(createdPatient).ToNot(BeNil())
					Expect(created).To(BeTrue())
				})

				It("creates the patient in the repository when there is no hard limit in the patient count settings", func() {
					patientCountSettings.HardLimit = nil

					clinicsService.EXPECT().
						GetPatientCountSettings(gomock.Any(), gomock.Eq(clinicId.Hex())).
						Return(patientCountSettings, nil)
					repo.EXPECT().
						Create(gomock.Any(), gomock.Eq(randomPatient)).
						Return(&randomPatient, true, nil)
					repo.EXPECT().
						Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: &clinicIdString, ExcludeDemo: true})).
						Return(patientCount.PatientCount, nil)
					clinicsService.EXPECT().
						UpdatePatientCount(gomock.Any(), gomock.Eq(clinicIdString), gomock.Eq(patientCount)).
						Return(nil)

					createdPatient, created, err := service.Create(context.Background(), randomPatient)
					Expect(err).ToNot(HaveOccurred())
					Expect(createdPatient).ToNot(BeNil())
					Expect(created).To(BeTrue())
				})

				It("creates the patient in the repository when the start date is after now in the hard limit in the patient count settings", func() {
					patientCountSettings.HardLimit.StartDate = Ptr(now.Add(time.Minute))

					clinicsService.EXPECT().
						GetPatientCountSettings(gomock.Any(), gomock.Eq(clinicId.Hex())).
						Return(patientCountSettings, nil)
					repo.EXPECT().
						Create(gomock.Any(), gomock.Eq(randomPatient)).
						Return(&randomPatient, true, nil)
					repo.EXPECT().
						Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: &clinicIdString, ExcludeDemo: true})).
						Return(patientCount.PatientCount, nil)
					clinicsService.EXPECT().
						UpdatePatientCount(gomock.Any(), gomock.Eq(clinicIdString), gomock.Eq(patientCount)).
						Return(nil)

					createdPatient, created, err := service.Create(context.Background(), randomPatient)
					Expect(err).ToNot(HaveOccurred())
					Expect(createdPatient).ToNot(BeNil())
					Expect(created).To(BeTrue())
				})

				It("creates the patient in the repository when the end date is before now in the hard limit in the patient count settings", func() {
					patientCountSettings.HardLimit.EndDate = Ptr(now.Add(-time.Minute))

					clinicsService.EXPECT().
						GetPatientCountSettings(gomock.Any(), gomock.Eq(clinicId.Hex())).
						Return(patientCountSettings, nil)
					repo.EXPECT().
						Create(gomock.Any(), gomock.Eq(randomPatient)).
						Return(&randomPatient, true, nil)
					repo.EXPECT().
						Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: &clinicIdString, ExcludeDemo: true})).
						Return(patientCount.PatientCount, nil)
					clinicsService.EXPECT().
						UpdatePatientCount(gomock.Any(), gomock.Eq(clinicIdString), gomock.Eq(patientCount)).
						Return(nil)

					createdPatient, created, err := service.Create(context.Background(), randomPatient)
					Expect(err).ToNot(HaveOccurred())
					Expect(createdPatient).ToNot(BeNil())
					Expect(created).To(BeTrue())
				})

				When("the patient count settings are returned", func() {
					BeforeEach(func() {
						clinicsService.EXPECT().
							GetPatientCountSettings(gomock.Any(), gomock.Eq(clinicId.Hex())).
							Return(patientCountSettings, nil)
					})

					It("returns an error when GetPatientCount returns an error", func() {
						testErr := fmt.Errorf("test error")

						clinicsService.EXPECT().
							GetPatientCount(gomock.Any(), gomock.Eq(clinicId.Hex())).
							Return(nil, testErr)

						createdPatient, created, err := service.Create(context.Background(), randomPatient)
						Expect(err).To(Equal(testErr))
						Expect(createdPatient).To(BeNil())
						Expect(created).To(BeFalse())
					})

					It("returns an error when there is no patient count", func() {
						clinicsService.EXPECT().
							GetPatientCount(gomock.Any(), gomock.Eq(clinicId.Hex())).
							Return(nil, nil)

						createdPatient, created, err := service.Create(context.Background(), randomPatient)
						Expect(err).To(MatchError(errors.InternalServerError))
						Expect(createdPatient).To(BeNil())
						Expect(created).To(BeFalse())
					})

					It("returns an error when patient count is greater than or equal to the hard limit", func() {
						patientCount.PatientCount = patientCountSettings.HardLimit.PatientCount

						clinicsService.EXPECT().
							GetPatientCount(gomock.Any(), gomock.Eq(clinicId.Hex())).
							Return(patientCount, nil)

						createdPatient, created, err := service.Create(context.Background(), randomPatient)
						Expect(err).To(MatchError(errors.PaymentRequired))
						Expect(createdPatient).To(BeNil())
						Expect(created).To(BeFalse())
					})

					When("the patient count is returned and the patient count is less than the hard limit", func() {
						BeforeEach(func() {
							clinicsService.EXPECT().
								GetPatientCount(gomock.Any(), gomock.Eq(clinicId.Hex())).
								Return(patientCount, nil)
						})

						It("does not create the patient in the repository", func() {
							repo.EXPECT().
								Create(gomock.Any(), gomock.Eq(randomPatient)).
								Return(&randomPatient, false, nil)

							createdPatient, created, err := service.Create(context.Background(), randomPatient)
							Expect(err).ToNot(HaveOccurred())
							Expect(createdPatient).ToNot(BeNil())
							Expect(created).To(BeFalse())
						})

						It("creates the patient in the repository", func() {
							repo.EXPECT().
								Create(gomock.Any(), gomock.Eq(randomPatient)).
								Return(&randomPatient, true, nil)
							repo.EXPECT().
								Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: &clinicIdString, ExcludeDemo: true})).
								Return(patientCount.PatientCount, nil)
							clinicsService.EXPECT().
								UpdatePatientCount(gomock.Any(), gomock.Eq(clinicIdString), gomock.Eq(patientCount)).
								Return(nil)

							createdPatient, created, err := service.Create(context.Background(), randomPatient)
							Expect(err).ToNot(HaveOccurred())
							Expect(createdPatient).ToNot(BeNil())
							Expect(created).To(BeTrue())
						})
					})
				})
			})
		})
	})

	Describe("Update", func() {
		var update patients.PatientUpdate

		BeforeEach(func() {
			update = patientsTest.RandomPatientUpdate()
			update.Patient.Permissions = &patients.Permissions{
				Upload: &patients.Permission{},
			}
		})

		When("the clinic requires the mrn to be set", func() {
			BeforeEach(func() {
				repo.
					EXPECT().
					Get(gomock.Any(), gomock.Eq(update.ClinicId), gomock.Eq(update.UserId)).
					Return(&update.Patient, nil)
				clinicsService.
					EXPECT().
					GetMRNSettings(gomock.Any(), gomock.Eq(update.ClinicId)).
					Return(&clinics.MRNSettings{Required: true}, nil)
			})

			It("updates the patient in the repository when the MRN is set", func() {
				repo.EXPECT().
					Update(gomock.Any(), gomock.Eq(update)).
					Return(&update.Patient, nil)

				updatedPatient, err := service.Update(context.Background(), update)
				Expect(err).To(BeNil())
				Expect(updatedPatient).ToNot(BeNil())
			})

			It("returns an error when the MRN is not set", func() {
				update.Patient.Mrn = nil

				createdPatient, err := service.Update(context.Background(), update)
				Expect(err).To(MatchError(errors.BadRequest))
				Expect(createdPatient).To(BeNil())
			})
		})

		When("the clinic requires mrn to be unique", func() {
			BeforeEach(func() {
				repo.
					EXPECT().
					Get(gomock.Any(), gomock.Eq(update.ClinicId), gomock.Eq(update.UserId)).
					Return(&update.Patient, nil)
				clinicsService.
					EXPECT().
					GetMRNSettings(gomock.Any(), gomock.Eq(update.ClinicId)).
					Return(&clinics.MRNSettings{Unique: true}, nil)
			})

			It("updates the patient in the repository with uniqueness flag set to true", func() {
				expectedUpdate := update
				expectedUpdate.Patient.RequireUniqueMrn = true

				repo.EXPECT().
					Update(gomock.Any(), gomock.Eq(expectedUpdate)).
					Return(&update.Patient, nil)

				repo.EXPECT().
					List(gomock.Any(), &patients.Filter{ClinicId: &update.ClinicId, Mrn: update.Patient.Mrn}, gomock.Any(), gomock.Any()).
					Return(&patients.ListResult{
						Patients:   nil,
						TotalCount: 0,
					}, nil)

				updatedPatient, err := service.Update(context.Background(), update)
				Expect(err).To(BeNil())
				Expect(updatedPatient).ToNot(BeNil())
			})

			It("returns an error if a patient with the same mrn exists in the repository", func() {
				existing := patientsTest.RandomPatient()
				existing.Mrn = update.Patient.Mrn

				repo.EXPECT().
					List(gomock.Any(), &patients.Filter{ClinicId: &update.ClinicId, Mrn: update.Patient.Mrn}, gomock.Any(), gomock.Any()).
					Return(&patients.ListResult{
						Patients:   []*patients.Patient{&existing},
						TotalCount: 1,
					}, nil)

				updatedPatient, err := service.Update(context.Background(), update)
				Expect(err).To(MatchError("bad request: mrn must be unique"))
				Expect(updatedPatient).To(BeNil())
			})
		})

		When("there are active subscriptions", func() {
			var randomPatient patients.Patient

			BeforeEach(func() {
				randomPatient = patientsTest.RandomPatient()
				randomPatient.EHRSubscriptions = patientsTest.RandomSubscriptions()
				randomPatient.Permissions.Custodian = nil

				repo.
					EXPECT().
					Get(gomock.Any(), gomock.Eq(update.ClinicId), gomock.Eq(update.UserId)).
					Return(&randomPatient, nil)
				clinicsService.
					EXPECT().
					GetMRNSettings(gomock.Any(), gomock.Eq(update.ClinicId)).
					Return(nil, nil)
			})

			It("deactivates subscriptions if patients mrn has changed", func() {
				repo.EXPECT().
					Update(gomock.Any(), gomock.All(test.Match[patients.PatientUpdate](func(update patients.PatientUpdate) bool {
						if len(update.Patient.EHRSubscriptions) == 0 {
							return false
						}
						for _, sub := range update.Patient.EHRSubscriptions {
							if sub.Active == true {
								return false
							}
						}
						return true
					}))).Return(&update.Patient, nil)

				updatedPatient, err := service.Update(context.Background(), update)
				Expect(err).To(BeNil())
				Expect(updatedPatient).ToNot(BeNil())
			})
		})
	})

	Describe("Update Permissions", func() {
		Context("With non-empty permissions", func() {
			perms := &patients.Permissions{
				Upload: &patients.Permission{},
			}

			It("updates permissions in repository", func() {
				userId := "1234567890"
				clinicId := "60d1dc0eac5285751add8f82"
				repo.EXPECT().
					UpdatePermissions(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId), gomock.Eq(perms)).
					Return(&patients.Patient{Permissions: perms}, nil)

				_, err := service.UpdatePermissions(context.Background(), clinicId, userId, perms)
				Expect(err).To(BeNil())
			})
		})

		Context("With custodian permission", func() {
			perms := &patients.Permissions{
				Custodian: &patients.Permission{},
			}

			It("removes the patient from the repository", func() {
				userId := "1234567890"
				clinicId := "60d1dc0eac5285751add8f82"
				patientCount := &clinics.PatientCount{PatientCount: 10}

				repo.EXPECT().
					Remove(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId)).
					Return(nil)
				repo.EXPECT().
					Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: &clinicId, ExcludeDemo: true})).
					Return(patientCount.PatientCount, nil)
				clinicsService.EXPECT().
					UpdatePatientCount(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(patientCount)).
					Return(nil)

				patient, err := service.UpdatePermissions(context.Background(), clinicId, userId, perms)
				Expect(patient).To(BeNil())
				Expect(err).To(BeNil())
			})
		})

		Context("With empty permissions", func() {
			perms := &patients.Permissions{}

			It("removes the patient from the repository", func() {
				userId := "1234567890"
				clinicId := "60d1dc0eac5285751add8f82"
				patientCount := &clinics.PatientCount{PatientCount: 10}

				repo.EXPECT().
					Remove(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId)).
					Return(nil)
				repo.EXPECT().
					Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: &clinicId, ExcludeDemo: true})).
					Return(patientCount.PatientCount, nil)
				clinicsService.EXPECT().
					UpdatePatientCount(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(patientCount)).
					Return(nil)

				patient, err := service.UpdatePermissions(context.Background(), clinicId, userId, perms)
				Expect(patient).To(BeNil())
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("Delete Permission", func() {
		permission := "upload"

		Context("With non-empty permissions post update", func() {
			It("removes the patient permissions from the repository", func() {
				userId := "1234567890"
				clinicId := "60d1dc0eac5285751add8f82"
				repo.EXPECT().
					DeletePermission(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId), gomock.Eq(permission)).
					Return(&patients.Patient{Permissions: &patients.Permissions{
						View: &patients.Permission{},
					}}, nil)

				patient, err := service.DeletePermission(context.Background(), clinicId, userId, permission)
				Expect(patient).ToNot(BeNil())
				Expect(err).To(BeNil())
			})
		})

		Context("With empty permissions post update", func() {
			It("removes the patient from the repository", func() {
				userId := "1234567890"
				clinicId := "60d1dc0eac5285751add8f82"
				patientCount := &clinics.PatientCount{PatientCount: 10}

				repo.EXPECT().
					DeletePermission(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId), gomock.Eq(permission)).
					Return(&patients.Patient{Permissions: &patients.Permissions{}}, nil)

				repo.EXPECT().
					Remove(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId)).
					Return(nil)
				repo.EXPECT().
					Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: &clinicId, ExcludeDemo: true})).
					Return(patientCount.PatientCount, nil)
				clinicsService.EXPECT().
					UpdatePatientCount(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(patientCount)).
					Return(nil)

				patient, err := service.DeletePermission(context.Background(), clinicId, userId, permission)
				Expect(patient).To(BeNil())
				Expect(err).To(BeNil())
			})

			It("doesn't return an error if the patient is already removed", func() {
				userId := "1234567890"
				clinicId := "60d1dc0eac5285751add8f82"

				repo.EXPECT().
					DeletePermission(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId), gomock.Eq(permission)).
					Return(&patients.Patient{Permissions: &patients.Permissions{}}, nil)

				repo.EXPECT().
					Remove(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId)).
					Return(patients.ErrNotFound)

				patient, err := service.DeletePermission(context.Background(), clinicId, userId, permission)
				Expect(patient).To(BeNil())
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("DeleteFromAllClinics", func() {
		It("delete the user from no clinics", func() {
			userId := "1234567890"

			repo.EXPECT().
				DeleteFromAllClinics(gomock.Any(), gomock.Eq(userId)).
				Return([]string{}, nil)

			clinicIds, err := service.DeleteFromAllClinics(context.Background(), userId)
			Expect(clinicIds).To(Equal([]string{}))
			Expect(err).To(BeNil())
		})

		It("delete the user from all clinics", func() {
			userId := "1234567890"
			expectedClinicIds := []string{"111111111111111111111111", "222222222222222222222222", "333333333333333333333333"}

			repo.EXPECT().
				DeleteFromAllClinics(gomock.Any(), gomock.Eq(userId)).
				Return(expectedClinicIds, nil)
			for index, expectedClinicId := range expectedClinicIds {
				repo.EXPECT().
					Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: Ptr(expectedClinicId), ExcludeDemo: true})).
					Return(index, nil)
				clinicsService.EXPECT().
					UpdatePatientCount(gomock.Any(), gomock.Eq(expectedClinicId), gomock.Eq(&clinics.PatientCount{PatientCount: index})).
					Return(nil)
			}

			clinicIds, err := service.DeleteFromAllClinics(context.Background(), userId)
			Expect(err).To(BeNil())
			Expect(clinicIds).To(Equal(expectedClinicIds))
		})
	})

	Describe("DeleteNonCustodialPatientsOfClinic", func() {
		It("delete no non-custodial patients of clinic", func() {
			clinicId := "1234567890"

			repo.EXPECT().
				DeleteNonCustodialPatientsOfClinic(gomock.Any(), gomock.Eq(clinicId)).
				Return(false, nil)

			deleted, err := service.DeleteNonCustodialPatientsOfClinic(context.Background(), clinicId)
			Expect(err).To(BeNil())
			Expect(deleted).To(BeFalse())
		})

		It("delete one or more non-custodial patients of clinic", func() {
			clinicId := "1234567890"
			patientCount := &clinics.PatientCount{PatientCount: 10}

			repo.EXPECT().
				DeleteNonCustodialPatientsOfClinic(gomock.Any(), gomock.Eq(clinicId)).
				Return(true, nil)
			repo.EXPECT().
				Count(gomock.Any(), gomock.Eq(&patients.Filter{ClinicId: Ptr(clinicId), ExcludeDemo: true})).
				Return(patientCount.PatientCount, nil)
			clinicsService.EXPECT().
				UpdatePatientCount(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(patientCount)).
				Return(nil)

			deleted, err := service.DeleteNonCustodialPatientsOfClinic(context.Background(), clinicId)
			Expect(err).To(BeNil())
			Expect(deleted).To(BeTrue())
		})
	})
})
