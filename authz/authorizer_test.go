package authz_test

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/authz"
	"go.uber.org/zap"
)

var clinicAdmin = map[string]interface{}{
	"roles": []string{"CLINIC_ADMIN"},
}

var clinicMember = map[string]interface{}{
	"roles": []string{"CLINIC_MEMBER"},
}

var _ = Describe("Request Authorizer", func() {
	var authorizer authz.RequestAuthorizer

	BeforeEach(func() {
		var err error
		authorizer, err = authz.NewRequestAuthorizer(nil, zap.NewNop().Sugar())
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Evaluate policy", func() {
		It("prevents users from accessing /v1/clinics", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics"},
				"method": "GET",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "1234567890",
					"x-auth-server-access": "false",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("allows hydrophone to access /v1/clinics", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics"},
				"method": "GET",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "hydrophone",
					"x-auth-server-access": "true",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("prevents random services from accessing /v1/clinics", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics"},
				"method": "GET",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "non-existent-service",
					"x-auth-server-access": "true",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("allows hydrophone to access /v1/clinics/6066fbabc6f484277200ac64", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64"},
				"method": "GET",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "hydrophone",
					"x-auth-server-access": "true",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows authenticated users to access a clinic by id", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64"},
				"method": "GET",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "123456",
					"x-auth-server-access": "false",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows authenticated users to fetch clinics by share code /v1/clinics/share_code/acmeclinic", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "share_code", "acmeclinic"},
				"method": "GET",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "123456",
					"x-auth-server-access": "false",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows shoreline to list clinics for a given user id", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "patients", "12345", "clinics"},
				"method": "GET",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "shoreline",
					"x-auth-server-access": "true",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows shoreline to delete custodian permission", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345", "permissions", "custodian"},
				"method": "DELETE",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "shoreline",
					"x-auth-server-access": "true",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allow users to delete permissions they have granted", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345", "permissions", "upload"},
				"method": "DELETE",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "12345",
					"x-auth-server-access": "false",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allow clinic admins to delete patients of a clinic", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345"},
				"method": "DELETE",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "999999999",
					"x-auth-server-access": "false",
				},
				"clinician": clinicAdmin,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("prevents users to migrate patients to a clinic", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345"},
				"method": "POST",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "999999999",
					"x-auth-server-access": "false",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("prevents users to migrate their own account to a clinic", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "999999999"},
				"method": "POST",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "999999999",
					"x-auth-server-access": "false",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("prevents clinic admins to migrate users to a clinic", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "999999999"},
				"method": "POST",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "999999999",
					"x-auth-server-access": "false",
				},
				"clinician": clinicAdmin,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("prevents clinic members to migrate users to a clinic", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "999999999"},
				"method": "POST",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "999999999",
					"x-auth-server-access": "false",
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("prevents clinic members from changing patient permissions", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "999999999", "permissions"},
				"method": "POST",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "999999999",
					"x-auth-server-access": "false",
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("prevents clinic admins from changing patient permissions", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "999999999", "permissions"},
				"method": "POST",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "999999999",
					"x-auth-server-access": "false",
				},
				"clinician": clinicAdmin,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("prevents clinic members to delete patients of a clinic", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345"},
				"method": "DELETE",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "999999999",
					"x-auth-server-access": "false",
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("allows users to delete their own patient profile", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345"},
				"method": "DELETE",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "12345",
					"x-auth-server-access": "false",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("prevents users to delete patient profiles of other people", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345"},
				"method": "DELETE",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "999999999",
					"x-auth-server-access": "false",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("does not allow other users to delete permissions", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345", "permissions", "upload"},
				"method": "DELETE",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "99999",
					"x-auth-server-access": "false",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(HaveOccurred())
		})

		It("allows hydrophone to access /v1/clinics/6066fbabc6f484277200ac64/clinicians", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "clinicians"},
				"method": "GET",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "hydrophone",
					"x-auth-server-access": "true",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows clinicians to list clinics they are a member of", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinicians", "1234567890", "clinics"},
				"method": "GET",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "1234567890",
					"x-auth-server-access": "false",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows clinic admins to update patients", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "99c290f838"},
				"method": "PUT",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "1234567890",
					"x-auth-server-access": "false",
				},
				"clinician": clinicAdmin,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows clinic admins to create custodial accounts for patients", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients"},
				"method": "POST",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "1234567890",
					"x-auth-server-access": "false",
				},
				"clinician": clinicAdmin,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it prevents clinic members to update patients", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "99c290f838"},
				"method": "PUT",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "1234567890",
					"x-auth-server-access": "false",
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("it allows currently authenticated clinic member to delete their own profile", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "clinicians", "1234567890"},
				"method": "DELETE",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "1234567890",
					"x-auth-server-access": "false",
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it prevents currently authenticated clinic member to delete profiles of other clinicians", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "clinicians", "99999999"},
				"method": "DELETE",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "1234567890",
					"x-auth-server-access": "false",
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("it prevents clinicians to list clinics of other users", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinicians", "123456789", "clinics"},
				"method": "GET",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "1234567890",
					"x-auth-server-access": "false",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(authz.ErrUnauthorized))
		})

		It("it allows hydrophone to update invited clinicians", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "invites", "clinicians", "gw94dmVOaB4CH", "clinician"},
				"method": "PATCH",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "hydrophone",
					"x-auth-server-access": "true",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows hydrophone to retrieve invited clinicians", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "invites", "clinicians", "gw94dmVOaB4CH", "clinician"},
				"method": "GET",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "hydrophone",
					"x-auth-server-access": "true",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows hydrophone to delete invited clinicians", func() {
			input := map[string]interface{}{
				"path": []string{"v1", "clinics", "6066fbabc6f484277200ac64", "invites", "clinicians", "gw94dmVOaB4CH", "clinician"},
				"method": "DELETE",
				"headers": map[string]interface{}{
					"x-auth-subject-id": "hydrophone",
					"x-auth-server-access": "true",
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
