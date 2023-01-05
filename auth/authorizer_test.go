package auth_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/auth"
	"go.uber.org/zap"
)

var clinicAdmin = map[string]interface{}{
	"roles": []string{"CLINIC_ADMIN"},
}

var clinicMember = map[string]interface{}{
	"roles": []string{"CLINIC_MEMBER"},
}

var _ = Describe("Request Authorizer", func() {
	var authorizer auth.RequestAuthorizer

	BeforeEach(func() {
		var err error
		authorizer, err = auth.NewRequestAuthorizer(nil, zap.NewNop().Sugar())
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Evaluate policy", func() {
		It("prevents users from accessing /v1/clinics", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("allows hydrophone to access /v1/clinics", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "hydrophone",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows hydrophone to access /v1/clinics/6066fbabc6f484277200ac64", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "hydrophone",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows authenticated users to access a clinic by id", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "123456",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows authenticated users to fetch clinics by share code /v1/clinics/share_code/acmeclinic", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "share_code", "acmeclinic"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "123456",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows shoreline to list clinics for a given user id", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "patients", "12345", "clinics"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "shoreline",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows shoreline to delete custodian permission", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345", "permissions", "custodian"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "shoreline",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allow users to delete permissions they have granted", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345", "permissions", "upload"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "12345",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allow clinic admins to delete patients of a clinic", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "999999999",
					"serverAccess": false,
				},
				"clinician": clinicAdmin,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("prevents users to migrate patients to a clinic", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "999999999",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("prevents users to migrate their own account to a clinic", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "999999999"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "999999999",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("prevents clinic admins to migrate users to a clinic", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "999999999"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "999999999",
					"serverAccess": false,
				},
				"clinician": clinicAdmin,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("prevents clinic members to migrate users to a clinic", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "999999999"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "999999999",
					"serverAccess": false,
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("prevents clinic members from changing patient permissions", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "999999999", "permissions"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "999999999",
					"serverAccess": false,
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("prevents clinic admins from changing patient permissions", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "999999999", "permissions"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "999999999",
					"serverAccess": false,
				},
				"clinician": clinicAdmin,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("prevents clinic members to delete patients of a clinic", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "999999999",
					"serverAccess": false,
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("allows users to delete their own patient profile", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "12345",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("prevents users to delete patient profiles of other people", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "999999999",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("does not allow other users to delete permissions", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "12345", "permissions", "upload"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "99999",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(HaveOccurred())
		})

		It("allows prescription service to fetch clinician by id", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "clinicians", "1234567890"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "prescription",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows hydrophone to access /v1/clinics/6066fbabc6f484277200ac64/clinicians", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "clinicians"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "hydrophone",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows clinicians to list clinics they are a member of", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinicians", "1234567890", "clinics"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows clinic admins to update patients", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "99c290f838"},
				"method": "PUT",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
				"clinician": clinicAdmin,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows clinic members to update patients", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "99c290f838"},
				"method": "PUT",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows clinic admins to create custodial accounts for patients", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
				"clinician": clinicAdmin,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows clinic members to create custodial accounts for patients", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows currently authenticated clinic member to delete their own profile", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "clinicians", "1234567890"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it prevents currently authenticated clinic member to delete profiles of other clinicians", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "clinicians", "99999999"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("it prevents clinicians to list clinics of other users", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinicians", "123456789", "clinics"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("it allows hydrophone to update invited clinicians", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "invites", "clinicians", "gw94dmVOaB4CH", "clinician"},
				"method": "PATCH",
				"auth": map[string]interface{}{
					"subjectId":    "hydrophone",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows hydrophone to retrieve invited clinicians", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "invites", "clinicians", "gw94dmVOaB4CH", "clinician"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "hydrophone",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows hydrophone to delete invited clinicians", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "invites", "clinicians", "gw94dmVOaB4CH", "clinician"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "hydrophone",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows orca to fetch migrations by id", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "migrations", "123456789"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "orca",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows clinic-worker to fetch migrations by id", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "migrations", "123456789"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "clinic-worker",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows clinicians to fetch their own migrations", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "migrations", "1234567890"},
				"method": "GET",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows clinic-worker to update migrations", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "migrations", "1234567890"},
				"method": "PATCH",
				"auth": map[string]interface{}{
					"subjectId":    "clinic-worker",
					"serverAccess": true,
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it prevents clinicians to update their own migrations", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "migrations", "1234567890"},
				"method": "PATCH",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("it allows clinic-worker to delete user accounts", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "users", "1234567890", "clinics"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "clinic-worker",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it allows orca to update clinic service tier", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "tier"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "clinic-worker",
					"serverAccess": true,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it prevents members to update clinic service tier", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "tier"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
				"clinician": clinicMember,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("it prevents admins to update clinic service tier", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "tier"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
				"clinician": clinicAdmin,
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("it prevents users to update clinic service tier", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "tier"},
				"method": "POST",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})

		It("it prevents users from triggering deletion", func() {
			input := map[string]interface{}{
				"path":   []string{"v1", "users", "1234567890", "clinics"},
				"method": "DELETE",
				"auth": map[string]interface{}{
					"subjectId":    "1234567890",
					"serverAccess": false,
				},
			}
			err := authorizer.EvaluatePolicy(context.Background(), input)
			Expect(err).To(Equal(auth.ErrUnauthorized))
		})
	})

	It("it allows currently authenticated clinic member to create patient tags", func() {
		input := map[string]interface{}{
			"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patient_tags"},
			"method": "POST",
			"auth": map[string]interface{}{
				"subjectId":    "1234567890",
				"serverAccess": false,
			},
			"clinician": clinicMember,
		}
		err := authorizer.EvaluatePolicy(context.Background(), input)
		Expect(err).ToNot(HaveOccurred())
	})

	It("it allows currently authenticated clinic member to update patient tags", func() {
		input := map[string]interface{}{
			"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patient_tags", "6066fbabc6f484277200ac65"},
			"method": "PUT",
			"auth": map[string]interface{}{
				"subjectId":    "1234567890",
				"serverAccess": false,
			},
			"clinician": clinicMember,
		}
		err := authorizer.EvaluatePolicy(context.Background(), input)
		Expect(err).ToNot(HaveOccurred())
	})

	It("it allows currently authenticated clinic admin to delete patient tags", func() {
		input := map[string]interface{}{
			"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patient_tags", "6066fbabc6f484277200ac65"},
			"method": "DELETE",
			"auth": map[string]interface{}{
				"subjectId":    "1234567890",
				"serverAccess": false,
			},
			"clinician": clinicAdmin,
		}
		err := authorizer.EvaluatePolicy(context.Background(), input)
		Expect(err).ToNot(HaveOccurred())
	})

	It("it prevents currently authenticated clinic non-admin member from deleting patient tags", func() {
		input := map[string]interface{}{
			"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patient_tags", "6066fbabc6f484277200ac65"},
			"method": "DELETE",
			"auth": map[string]interface{}{
				"subjectId":    "1234567890",
				"serverAccess": false,
			},
			"clinician": clinicMember,
		}
		err := authorizer.EvaluatePolicy(context.Background(), input)
		Expect(err).To(Equal(auth.ErrUnauthorized))
	})

	It("it allows clinic-worker to remove a tag from all matching patients", func() {
		input := map[string]interface{}{
			"path":   []string{"v1", "clinics", "6066fbabc6f484277200ac64", "patients", "delete_tag", "6066fbabc6f484277200ac65"},
			"method": "DELETE",
			"auth": map[string]interface{}{
				"subjectId":    "clinic-worker",
				"serverAccess": true,
			},
		}
		err := authorizer.EvaluatePolicy(context.Background(), input)
		Expect(err).ToNot(HaveOccurred())
	})

	It("it allows auth to decline a dexcom connect request for all matching patients", func() {
		input := map[string]interface{}{
			"path":   []string{"v1", "patients", "99c290f838", "decline_dexcom_connect_request"},
			"method": "POST",
			"auth": map[string]interface{}{
				"subjectId":    "auth",
				"serverAccess": true,
			},
		}
		err := authorizer.EvaluatePolicy(context.Background(), input)
		Expect(err).ToNot(HaveOccurred())
	})
})
