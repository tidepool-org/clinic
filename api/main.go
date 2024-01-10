package api

import (
	"context"
	"fmt"

	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/redox"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	oapiMiddleware "github.com/oapi-codegen/echo-middleware"
	"github.com/tidepool-org/clinic/auth"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	"github.com/tidepool-org/clinic/clinics/migration"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/logger"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
)

var (
	ServerString        = ":8080"
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

func SetReady(healthCheck *HealthCheck, db *mongo.Database, lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := db.Client().Ping(ctx, nil); err != nil {
				return err
			}

			// It's important this is set after mongo is initialized, which is ensured
			// by taking a dependency on mongo in the constructor, because lifecycle hooks
			// are executed in topological order
			healthCheck.SetReady(true)
			return nil
		},
		OnStop: nil,
	})
}

func NewServer(handler *Handler, healthCheck *HealthCheck, authorizer auth.RequestAuthorizer, authenticator auth.Authenticator) (*echo.Echo, error) {
	e := echo.New()
	e.Logger.Print("Starting Main Loop")
	swagger, err := GetSwagger()
	if err != nil {
		return nil, err
	}

	// Do not validate servers in the open api spec
	swagger.Servers = nil

	healthcheckRoutes := []string{"/ready", "/metrics"}
	redoxRoutes := []string{"/v1/redox", "/v1/redox/verify"}

	// Skip common auth logic for healthcheck routes and redox
	authMiddleware := auth.NewAuthMiddleware(authenticator, auth.AuthMiddlewareOpts{
		Skipper: RouteSkipper(append(healthcheckRoutes, redoxRoutes...)),
	})
	requestValidator := oapiMiddleware.OapiRequestValidatorWithOptions(swagger, &oapiMiddleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc:          authorizer.Authorize,
			ExcludeReadOnlyValidations:  true,
			ExcludeWriteOnlyValidations: true,
		},
		Skipper: RouteSkipper(append(healthcheckRoutes, redoxRoutes...)),
	})
	loggerConfig := middleware.DefaultLoggerConfig
	loggerConfig.Skipper = RouteSkipper(healthcheckRoutes)
	loggerMiddleware := middleware.LoggerWithConfig(loggerConfig)

	e.Use(middleware.Recover())
	e.Use(loggerMiddleware)
	e.Use(authMiddleware)
	e.Use(requestValidator)

	e.HTTPErrorHandler = errors.CustomHTTPErrorHandler

	e.GET("/ready", healthCheck.Ready)
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
			redox.NewConfig,
			redox.NewHandler,
			clinicians.NewRepository,
			clinicians.NewService,
			clinics.NewRepository,
			clinics.NewShareCodeGenerator,
			config.NewConfig,
			manager.NewManager,
			migration.NewMigrator,
			migration.NewRepository,
			auth.NewAuthenticator,
			auth.NewRequestAuthorizer,
			NewHealthCheck,
			NewHandler,
			NewServer,
		),
		patients.UserServiceModule,
		fx.Invoke(SetReady),
		fx.Invoke(Start),
	).Run()
}
