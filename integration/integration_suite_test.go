package integration_test

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/api"
	integrationTest "github.com/tidepool-org/clinic/integration/test"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/clinic/test"
	xealthTest "github.com/tidepool-org/clinic/xealth/test"
	"go.uber.org/fx"
	"net/http/httptest"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
)

var app *fx.App
var server *echo.Echo
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

	authStub = integrationTest.AuthStub()
	seagullStub = integrationTest.SeagullStub()
	shorelineStub = integrationTest.ShorelineStub()
	xealthStub = xealthTest.ServerStub()
	keycloakStub := integrationTest.KeycloakStub()

	t := GinkgoT()
	t.Setenv("LOG_LEVEL", "error")
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
	shorelineStub.Close()
	seagullStub.Close()

	if app != nil {
		Expect(app.Stop(context.Background())).To(Succeed())
	}
}
