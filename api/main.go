package api

import (
	"context"
	"fmt"
	"github.com/brpaz/echozap"
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
	"github.com/tidepool-org/clinic/redox"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/xealth"
	authClient "github.com/tidepool-org/platform/auth/client"
	"github.com/tidepool-org/platform/client"
	"github.com/tidepool-org/platform/platform"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
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

func NewServer(handler *Handler, healthCheck *HealthCheck, authorizer auth.RequestAuthorizer, authenticator auth.Authenticator, logger *zap.Logger) (*echo.Echo, error) {
	e := echo.New()
	logger.Info("Starting Main Loop")
	swagger, err := GetSwagger()
	if err != nil {
		return nil, err
	}

	// Do not validate servers in the open api spec
	swagger.Servers = nil

	healthcheckRoutes := []string{"/ready", "/metrics"}
	redoxRoutes := []string{"/v1/redox", "/v1/redox/verify"}
	xealthRoutes := []string{"/v1/xealth/preorder", "/v1/xealth/notification", "/v1/xealth/programs", "/v1/xealth/program"}
	externalRoutes := append(append(healthcheckRoutes, redoxRoutes...), xealthRoutes...)

	// Skip common auth logic for healthcheck routes, redox and xealth
	authMiddleware := auth.NewAuthMiddleware(authenticator, auth.AuthMiddlewareOpts{
		Skipper: RouteSkipper(externalRoutes),
	})
	requestValidator := oapiMiddleware.OapiRequestValidatorWithOptions(swagger, &oapiMiddleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc:          authorizer.Authorize,
			ExcludeReadOnlyValidations:  true,
			ExcludeWriteOnlyValidations: true,
		},
		Skipper: RouteSkipper(externalRoutes),
	})
	healthCheckSkipper := RouteSkipper(healthcheckRoutes)
	e.Use(middleware.Recover())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Do not log health check requests
			if healthCheckSkipper(c) {
				return next(c)
			}
			// Log all other requests with the zap middleware
			return echozap.ZapLogger(logger)(next)(c)
		}
	})
	//e.Use(loggerMiddleware)
	e.Use(authMiddleware)
	e.Use(requestValidator)

	e.HTTPErrorHandler = errors.CustomHTTPErrorHandler

	e.GET("/ready", healthCheck.Ready)
	RegisterHandlers(e, handler)

	return e, nil
}

func Dependencies() []fx.Option {
	return []fx.Option{
		auth.PlatformClientModule,
		fx.Provide(
			logger.NewProductionLogger,
			logger.Suggar,
			store.NewConfig,
			store.NewClient,
			store.NewDatabase,
			patients.NewRepository,
			patients.NewCustodialService,
			patients.NewService,
			redox.NewConfig,
			redox.NewHandler,
			xealth.NewStore,
			xealth.NewHandler,
			clinicians.NewRepository,
			clinicians.NewService,
			clinics.NewRepository,
			clinics.NewShareCodeGenerator,
			manager.NewConfig,
			manager.NewManager,
			migration.NewMigrator,
			migration.NewRepository,
			authClient.NewExternalEnvconfigLoader,
			platform.NewEnvconfigLoader,
			client.NewEnvconfigLoader,
			auth.NewAuthenticator,
			auth.NewRequestAuthorizer,
			NewHealthCheck,
			NewHandler,
			NewServer,
		),
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		patients.UserServiceModule,
	}
}

func MainLoop() {
	app := append(Dependencies(), fx.Invoke(SetReady), fx.Invoke(Start))
	fx.New(app...).Run()
}
