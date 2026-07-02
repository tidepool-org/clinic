package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/api"
	integrationTest "github.com/tidepool-org/clinic/integration/test"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
	xealthTest "github.com/tidepool-org/clinic/xealth/test"
	"go.uber.org/fx"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

const (
	XealthBearerToken      = "xealth-token"
	RedoxVerificationToken = "redox-token"
)

var app *fx.App
var server *echo.Echo
var stubUsers *integrationTest.StubUsers
var shorelineStub *httptest.Server
var seagullStub *httptest.Server
var authStub *httptest.Server
var xealthStub *xealthTest.XealthServer

func TestSuite(t *testing.T) {
	test.Test(t)
}

var _ = BeforeSuite(setupEnvironment)
var _ = AfterSuite(teardownEnvironment)

func setupEnvironment() {
	dbTest.SetupDatabase()
	dbTest.SetupPostgres()

	stubUsers = integrationTest.NewStubUsers()
	authStub = integrationTest.AuthStub()
	seagullStub = integrationTest.SeagullStub(stubUsers)
	shorelineStub = integrationTest.ShorelineStub(stubUsers)
	xealthStub = xealthTest.ServerStub()
	keycloakStub := integrationTest.KeycloakStub()

	t := GinkgoT()
	t.Setenv("LOG_LEVEL", "error")
	t.Setenv("TIDEPOOL_POSTGRES_ENABLED", "true")
	t.Setenv("TIDEPOOL_SERVER_TOKEN", integrationTest.TestServerToken)
	t.Setenv("TIDEPOOL_AUTH_CLIENT_EXTERNAL_SERVER_SESSION_TOKEN_SECRET", integrationTest.TestServerToken)
	t.Setenv("TIDEPOOL_AUTH_CLIENT_ADDRESS", authStub.URL)
	t.Setenv("TIDEPOOL_AUTH_CLIENT_EXTERNAL_ADDRESS", shorelineStub.URL)
	t.Setenv("TIDEPOOL_AUTH_SERVICE_TOKEN_ENDPOINT", keycloakStub.URL+"/realms/integration-test/protocol/openid-connect/token")
	t.Setenv("TIDEPOOL_SHORELINE_CLIENT_ADDRESS", shorelineStub.URL)
	t.Setenv("TIDEPOOL_SEAGULL_CLIENT_ADDRESS", seagullStub.URL)
	t.Setenv("TIDEPOOL_XEALTH_ENABLED", "true")
	t.Setenv("TIDEPOOL_XEALTH_BEARER_TOKEN", xealthTest.XealthBearerToken)
	t.Setenv("TIDEPOOL_XEALTH_CLIENT_ID", xealthTest.XealthClientId)
	t.Setenv("TIDEPOOL_XEALTH_CLIENT_SECRET", xealthTest.XealthClientSecret)
	t.Setenv("TIDEPOOL_XEALTH_SERVER_BASE_URL", xealthStub.URL)
	t.Setenv("TIDEPOOL_XEALTH_TOKEN_URL", fmt.Sprintf("%s%s", xealthStub.URL, xealthTest.TokenEndpoint))
	t.Setenv("TIDEPOOL_APPLICATION_URL", "https://integration.test.app.url.com")
	t.Setenv("TIDEPOOL_REDOX_VERIFICATION_TOKEN", RedoxVerificationToken)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	init := func(s *echo.Echo, lifecycle fx.Lifecycle) {
		lifecycle.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				wg.Done()
				return nil
			},
		})
		server = s
	}
	deps := append(api.Dependencies(), fx.Invoke(init))
	app = fx.New(deps...)
	go func() {
		_ = app.Start(context.Background())
	}()

	wg.Wait()
}

func teardownEnvironment() {
	dbTest.TeardownDatabase()
	dbTest.TeardownPostgres()
	shorelineStub.Close()
	seagullStub.Close()

	if app != nil {
		Expect(app.Stop(context.Background())).To(Succeed())
	}
}

func asClinician(req *http.Request) {
	req.Header.Set("x-tidepool-session-token", integrationTest.TestUserToken)
}

func asLegacyClinic(req *http.Request) {
	req.Header.Set("x-tidepool-session-token", integrationTest.TestLegacyClinicToken)
}

func asServer(req *http.Request) {
	req.Header.Set("x-tidepool-session-token", integrationTest.TestServerToken)
}

func asServiceAccount(req *http.Request) {
	req.Header.Set("x-tidepool-session-token", integrationTest.TestServiceAccountToken)
}

// asUser authenticates a request as any user registered with the stub user
// registry, e.g. one created via stubUsers.AddUser or newStubUser.
func asUser(userId string) func(*http.Request) {
	return func(req *http.Request) {
		GinkgoHelper()
		token := stubUsers.TokenFor(userId)
		Expect(token).ToNot(BeEmpty(), "user %s is not registered with the stub user registry", userId)
		req.Header.Set("x-tidepool-session-token", token)
	}
}

func asXealth(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", XealthBearerToken))
}

func asRedox(req *http.Request) {
	req.Header.Set("verification-token", RedoxVerificationToken)
}

func prepareRequest(method, endpoint string, fixturePath string) *http.Request {
	var body io.Reader
	if fixturePath != "" {
		b, err := test.LoadFixture(fixturePath)
		Expect(err).ToNot(HaveOccurred())
		body = bytes.NewReader(b)
	}

	return prepareRequestWithBody(method, endpoint, body)
}

func prepareRequestWithBody(method, endpoint string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, endpoint, body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	return req
}

func testCtx() context.Context {
	return context.Background()
}
