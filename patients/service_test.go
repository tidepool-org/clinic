package patients_test

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/patients/test"
)

var _ = Describe("Patients Service", func() {
	var service patients.Service
	var repo *test.MockRepository
	var repoCtrl *gomock.Controller

	BeforeEach(func() {
		repoCtrl = gomock.NewController(GinkgoT())
		repo = test.NewMockRepository(repoCtrl)

		var err error
		service, err = patients.NewService(repo, nil)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		repoCtrl.Finish()
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

				_, err := service.UpdatePermissions(nil, clinicId, userId, perms)
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
				repo.EXPECT().
					Remove(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId)).
					Return(nil)

				patient, err := service.UpdatePermissions(nil, clinicId, userId, perms)
				Expect(patient).To(BeNil())
				Expect(err).To(BeNil())
			})
		})

		Context("With empty permissions", func() {
			perms := &patients.Permissions{}

			It("removes the patient from the repository", func() {
				userId := "1234567890"
				clinicId := "60d1dc0eac5285751add8f82"
				repo.EXPECT().
					Remove(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId)).
					Return(nil)

				patient, err := service.UpdatePermissions(nil, clinicId, userId, perms)
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

				patient, err := service.DeletePermission(nil, clinicId, userId, permission)
				Expect(patient).ToNot(BeNil())
				Expect(err).To(BeNil())
			})
		})

		Context("With empty permissions post update", func() {
			It("removes the patient from the repository", func() {
				userId := "1234567890"
				clinicId := "60d1dc0eac5285751add8f82"

				repo.EXPECT().
					DeletePermission(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId), gomock.Eq(permission)).
					Return(&patients.Patient{Permissions: &patients.Permissions{}}, nil)

				repo.EXPECT().
					Remove(gomock.Any(), gomock.Eq(clinicId), gomock.Eq(userId)).
					Return(nil)

				patient, err := service.DeletePermission(nil, clinicId, userId, permission)
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

				patient, err := service.DeletePermission(nil, clinicId, userId, permission)
				Expect(patient).To(BeNil())
				Expect(err).To(BeNil())
			})
		})
	})
})
