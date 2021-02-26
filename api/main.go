package api

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/users"
	"go.uber.org/fx"
	"net/http"
)

var (
	Host = "localhost"
	Port = 8080
	ServerString = fmt.Sprintf("%s:%d", Host, Port)
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

func NewServer(handler *Handler) (*echo.Echo, error){
	e := echo.New()
	e.Logger.Print("Starting Main Loop")
	_, err := GetSwagger()
	if err != nil {
		return nil, err
	}

	// Middleware
	//authClient := AuthClient{Store: dbstore}
	//filterOptions := openapi3filter.Options{AuthenticationFunc: authClient.AuthenticationFunc}
	//options := Options{Options: filterOptions}
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	//e.Use(OapiRequestValidator(swagger, &Options{}))

	// Routes
	e.GET("/", hello)

	e.HTTPErrorHandler = errors.CustomHTTPErrorHandler

	RegisterHandlers(e, handler)

	return e, nil
}

func MainLoop() {
	fx.New(
		fx.Provide(
			store.GetConnectionString,
			store.NewDatabase,
			patients.NewRepository,
			clinics.NewRepository,
			clinicians.NewRepository,
			NewHandler,
			NewServer,
		),
		users.Module,
		fx.Invoke(Start),
	).Run()
}

// Handler
func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}


