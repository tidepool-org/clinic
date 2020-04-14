package api


import (
	"fmt"
	"github.com/tidepool-org/clinic/store"
	"net/http"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	Host = "localhost"
	Port = 3000
	ServerString = fmt.Sprintf("%s:%d", Host, Port)

)


func MainLoop() {
	// Echo instance
	e := echo.New()
	swagger, err := GetSwagger()
	if err != nil {
		e.Logger.Fatal("Cound not get spec")
	}

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(OapiRequestValidator(swagger))

	// Routes
	e.GET("/", hello)

	// Create Store
	store := store.NewMongoStoreClient()

	// Register Handler
	RegisterHandlers(e, &ClinicServer{store: store})

	// Start server
	fmt.Printf(ServerString)
	e.Logger.Fatal(e.Start(ServerString))
}

// Handler
func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}