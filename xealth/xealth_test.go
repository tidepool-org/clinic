package xealth_test

import (
	"context"
	"errors"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinics"
	clinicsTest "github.com/tidepool-org/clinic/clinics/test"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/xealth"
	"github.com/tidepool-org/platform/auth"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// fakeAuthClient returns a fixed restricted token.
type fakeAuthClient struct {
	auth.Client
	token *auth.RestrictedToken
}

func (f *fakeAuthClient) ServerSessionToken() (string, error) {
	return "server-session-token", nil
}

func (f *fakeAuthClient) GetRestrictedToken(ctx context.Context, id string) (*auth.RestrictedToken, error) {
	return f.token, nil
}

var _ = Describe("GetPDFReport", func() {
	const patientId = "1234567890"
	const restrictedToken = "restricted-token"

	var users *patientsTest.MockUserService
	var handler xealth.Xealth
	var request xealth.PDFReportRequest

	strp := func(s string) *string { return &s }

	getReportUrl := func() *url.URL {
		report, err := handler.GetPDFReport(context.Background(), request)
		Expect(err).ToNot(HaveOccurred())
		u, err := url.Parse(report.PdfUrl)
		Expect(err).ToNot(HaveOccurred())
		Expect(u.Path).To(Equal("/export/report/" + patientId))
		return u
	}

	BeforeEach(func() {
		t := GinkgoT()
		t.Setenv("TIDEPOOL_XEALTH_ENABLED", "true")
		t.Setenv("TIDEPOOL_XEALTH_BEARER_TOKEN", "bearer-token")
		t.Setenv("TIDEPOOL_XEALTH_CLIENT_ID", "client-id")
		t.Setenv("TIDEPOOL_XEALTH_CLIENT_SECRET", "client-secret")
		t.Setenv("TIDEPOOL_APPLICATION_URL", "https://example.com")

		clinicId := primitive.NewObjectID()
		request = xealth.PDFReportRequest{
			ClinicId:        clinicId.Hex(),
			PatientId:       patientId,
			RestrictedToken: restrictedToken,
		}

		ctrl := gomock.NewController(t)
		clinicsService := clinicsTest.NewMockService(ctrl)
		clinicsService.EXPECT().Get(gomock.Any(), request.ClinicId).Return(&clinics.Clinic{
			Id:          &clinicId,
			EHRSettings: &clinics.EHRSettings{Enabled: true},
		}, nil)
		patientsService := patientsTest.NewMockService(ctrl)
		patientsService.EXPECT().Get(gomock.Any(), request.ClinicId, patientId).Return(&patients.Patient{
			UserId:    strp(patientId),
			BirthDate: strp("2000-01-01"),
			Mrn:       strp("mrn-1"),
			FullName:  strp("Test Patient"),
			EHRSubscriptions: patients.EHRSubscriptions{
				patients.SubscriptionXealthReports: {Active: true},
			},
		}, nil)
		users = patientsTest.NewMockUserService(ctrl)

		authClient := &fakeAuthClient{token: &auth.RestrictedToken{
			ID:             restrictedToken,
			UserID:         patientId,
			ExpirationTime: time.Now().Add(time.Hour),
		}}

		var err error
		handler, err = xealth.NewHandler(authClient, clinicsService, patientsService, users, nil, zap.NewNop().Sugar())
		Expect(err).ToNot(HaveOccurred())
	})

	It("adds the patient's saved site change type to the report url", func() {
		users.EXPECT().GetUserSettings(gomock.Any(), patientId).Return(&patients.UserSettings{SiteChangeSource: "tubingPrime"}, nil)

		Expect(getReportUrl().Query()["siteChangeSource"]).To(Equal([]string{"tubingPrime"}))
	})

	It("returns a report url without a site change type when none is saved", func() {
		users.EXPECT().GetUserSettings(gomock.Any(), patientId).Return(&patients.UserSettings{}, nil)

		Expect(getReportUrl().Query()).ToNot(HaveKey("siteChangeSource"))
	})

	It("returns a report url without a site change type when the settings can't be fetched", func() {
		users.EXPECT().GetUserSettings(gomock.Any(), patientId).Return(nil, errors.New("seagull unavailable"))

		Expect(getReportUrl().Query()).ToNot(HaveKey("siteChangeSource"))
	})
})
