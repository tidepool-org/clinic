package authz_test

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/authz"
	"go.uber.org/zap"
)

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

	})
})
