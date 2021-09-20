package api

import (
	"context"
	"fmt"
	oapiMiddleware "github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/tidepool-org/clinic/authz"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/creator"
	"github.com/tidepool-org/clinic/clinics/migration"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/logger"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.uber.org/fx"
	"net/http"
)

var (
	Host                = "localhost"
	Port                = 8080
	ServerString        = fmt.Sprintf("%s:%d", Host, Port)
	ServerTimeoutAmount = 20
)

func Start(e *echo.Echo, lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := e.Start(ServerString); err != nil {
					fmt.Println(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return e.Shutdown(ctx)
		},
	})
}

func NewServer(handler *Handler, authorizer authz.RequestAuthorizer) (*echo.Echo, error) {
	e := echo.New()
	e.Logger.Print("Starting Main Loop")
	swagger, err := GetSwagger()
	if err != nil {
		return nil, err
	}

	// Do not validate servers
	swagger.Servers = nil
	requestValidator := oapiMiddleware.OapiRequestValidatorWithOptions(swagger, &oapiMiddleware.Options{
		Options: openapi3filter.Options{AuthenticationFunc: authorizer.Authorize},
	})

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(requestValidator)

	// Routes
	e.GET("/", hello)

	e.HTTPErrorHandler = errors.CustomHTTPErrorHandler

	RegisterHandlers(e, handler)

	return e, nil
}

func MainLoop() {
	fx.New(
		fx.Provide(
			logger.NewProductionLogger,
			logger.Suggar,
			store.GetConnectionString,
			store.NewClient,
			store.NewDatabase,
			patients.NewRepository,
			patients.NewCustodialService,
			patients.NewService,
			clinicians.NewRepository,
			clinicians.NewService,
			clinics.NewRepository,
			clinics.NewShareCodeGenerator,
			creator.NewConfig,
			creator.NewCreator,
			migration.NewMigrator,
			migration.NewRepository,
			authz.NewRequestAuthorizer,
			NewHandler,
			NewServer,
		),
		patients.UserServiceModule,
		fx.Invoke(Start),
	).Run()
}

// Handler
func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}
