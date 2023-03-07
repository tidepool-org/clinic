package api

import (
	"context"
	"fmt"
	oapiMiddleware "github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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

	// Skip auth, validation and logging for readiness probe and metrics routes
	skipper := RouteSkipper([]string{"/ready", "/metrics"})
	authMiddleware := auth.NewAuthMiddleware(authenticator, auth.AuthMiddlewareOpts{
		Skipper: skipper,
	})
	requestValidator := oapiMiddleware.OapiRequestValidatorWithOptions(swagger, &oapiMiddleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: authorizer.Authorize,
		},
		Skipper: skipper,
	})
	loggerConfig := middleware.DefaultLoggerConfig
	loggerConfig.Skipper = skipper
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
			clinicians.NewRepository,
			clinicians.NewService,
			clinics.NewRepository,
			clinics.NewShareCodeGenerator,
			manager.NewConfig,
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
