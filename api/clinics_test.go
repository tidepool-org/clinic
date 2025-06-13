package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/labstack/echo/v4"
	ginkgo "github.com/onsi/ginkgo/v2" // ginkgo and gen_types both have Offset
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store"
)

var _ = ginkgo.Describe("CreateSite", func() {
	ginkgo.It("returns name and id", func() {
		ec, th := newAPITestHelper()

		err := th.Handler.CreateSite(ec, "")
		if err != nil {
			th.Errorf("expected nil error, got %s", err)
		}

		raw := th.ReadResponseSlice()
		if got := len(raw); got != 1 {
			th.Errorf("expected 1 site in response, got: %d", got)
		}
		site := raw[0]
		if name := site["name"]; name != "Foo" {
			th.Errorf("expected \"Foo\"; got %q", name)
		}
		id, found := site["id"]
		if !found || id == "" {
			th.Errorf("expected id to be present; got \"\"")
		}
	})
})

type apiTestHelper struct {
	Handler *Handler
	Rec     *httptest.ResponseRecorder

	ginkgo.FullGinkgoTInterface
}

func newAPITestHelper() (echo.Context, *apiTestHelper) {

	handler := &Handler{
		ClinicsManager: newTestManager(),
		Clinics:        newTestClinics(),
	}

	echoServer := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Foo"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ec := echoServer.NewContext(req, rec)

	return ec, &apiTestHelper{
		Handler:              handler,
		Rec:                  rec,
		FullGinkgoTInterface: ginkgo.GinkgoT(),
	}
}

func (a *apiTestHelper) ReadResponseObject() map[string]any {
	raw := map[string]any{}
	if err := json.Unmarshal(a.Rec.Body.Bytes(), &raw); err != nil {
		a.Errorf("expected nil error, got %s", err)
	}
	return raw
}

func (a *apiTestHelper) ReadResponseSlice() []map[string]any {
	raw := []map[string]any{}
	if err := json.Unmarshal(a.Rec.Body.Bytes(), &raw); err != nil {
		a.Errorf("expected nil error, got %s", err)
	}
	return raw
}

type testManager struct{}

func newTestManager() *testManager { return &testManager{} }

func (t *testManager) CreateClinic(ctx context.Context, create *manager.CreateClinic) (*clinics.Clinic, error) {
	panic("not implemented") // TODO: Implement
}

func (t *testManager) DeleteClinic(ctx context.Context, clinicId string) error {
	panic("not implemented") // TODO: Implement
}

func (t *testManager) GetClinicPatientCount(ctx context.Context, clinicId string) (*clinics.PatientCount, error) {
	panic("not implemented") // TODO: Implement
}

func (t *testManager) FinalizeMerge(ctx context.Context, sourceId string, targetId string) error {
	panic("not implemented") // TODO: Implement
}

func (t *testManager) CreateSite(_ context.Context, clinicId string, name string) error {
	return nil
}

func (t *testManager) DeleteSite(_ context.Context, clinicId string, siteId string) error {
	panic("not implemented") // TODO: Implement
}

func (t *testManager) GetWithPatientCounts(_ context.Context, clinicId string) (*clinics.Clinic, error) {
	panic("not implemented") // TODO: Implement
}

func (t *testManager) ListSitesWithPatientCounts(_ context.Context, clinicId string) ([]sites.Site, error) {
	return []sites.Site{
		{Name: "Foo", Id: primitive.NewObjectID()},
	}, nil
}

func (t *testManager) UpdateSite(_ context.Context, clinicId string, siteId string, site *sites.Site) error {
	panic("not implemented") // TODO: Implement
}

type testClinics struct{}

func newTestClinics() *testClinics { return &testClinics{} }

func (c *testClinics) Get(ctx context.Context, id string) (*clinics.Clinic, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) List(ctx context.Context, filter *clinics.Filter, pagination store.Pagination) ([]*clinics.Clinic, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) Create(ctx context.Context, clinic *clinics.Clinic) (*clinics.Clinic, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) Update(ctx context.Context, id string, clinic *clinics.Clinic) (*clinics.Clinic, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) Delete(ctx context.Context, id string) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) UpsertAdmin(ctx context.Context, clinicId string, clinicianId string) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) RemoveAdmin(ctx context.Context, clinicId string, clinicianId string, allowOrphaning bool) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) UpdateTier(ctx context.Context, clinicId string, tier string) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) UpdateSuppressedNotifications(ctx context.Context, clinicId string, suppressedNotifications clinics.SuppressedNotifications) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) CreatePatientTag(ctx context.Context, clinicId string, tagName string) (*clinics.Clinic, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) UpdatePatientTag(ctx context.Context, clinicId string, tagId string, tagName string) (*clinics.Clinic, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) DeletePatientTag(ctx context.Context, clinicId string, tagId string) (*clinics.Clinic, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) ListMembershipRestrictions(ctx context.Context, clinicId string) ([]clinics.MembershipRestrictions, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) UpdateMembershipRestrictions(ctx context.Context, clinicId string, restrictions []clinics.MembershipRestrictions) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) GetEHRSettings(ctx context.Context, clinicId string) (*clinics.EHRSettings, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) UpdateEHRSettings(ctx context.Context, clinicId string, settings *clinics.EHRSettings) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) GetMRNSettings(ctx context.Context, clinicId string) (*clinics.MRNSettings, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) UpdateMRNSettings(ctx context.Context, clinicId string, settings *clinics.MRNSettings) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) GetPatientCountSettings(ctx context.Context, clinicId string) (*clinics.PatientCountSettings, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) UpdatePatientCountSettings(ctx context.Context, clinicId string, settings *clinics.PatientCountSettings) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) GetPatientCount(ctx context.Context, clinicId string) (*clinics.PatientCount, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) UpdatePatientCount(ctx context.Context, clinicId string, patientCount *clinics.PatientCount) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) AppendShareCodes(ctx context.Context, clinicId string, shareCodes []string) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) CreateSite(ctx context.Context, clinicId string, site *sites.Site) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) DeleteSite(ctx context.Context, clinicId string, siteId string) error {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) ListSites(ctx context.Context, clinicId string) ([]sites.Site, error) {
	panic("not implemented") // TODO: Implement
}

func (c *testClinics) UpdateSite(ctx context.Context, clinicId string, siteId string, site *sites.Site) error {
	panic("not implemented") // TODO: Implement
}
