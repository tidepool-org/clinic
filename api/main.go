package api


import (
	"fmt"
	"github.com/tidepool-org/clinic/store"
	"net/http"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/getkin/kin-openapi/openapi3filter"
)

var (
	Host = "localhost"
	Port = 8080
	ServerString = fmt.Sprintf("%s:%d", Host, Port)

)


func MainLoop() {
	// Echo instance
	e := echo.New()
	e.Logger.Print("Starting Main Loop")
	swagger, err := GetSwagger()
	if err != nil {
		e.Logger.Fatal("Cound not get spec")
	}

	// Connection string
	mongoHost, err := store.GetConnectionString()
	if err != nil {
		e.Logger.Fatal("Cound not connect to database: ", err)
	}

	// Create Store
	e.Logger.Print("Getting Mongog Store")
	dbstore := store.NewMongoStoreClient(mongoHost)


	// Middleware
	authClient := AuthClient{store: dbstore}
	filterOptions := openapi3filter.Options{AuthenticationFunc: authClient.AuthenticationFunc}
	options := Options{Options: filterOptions}
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(OapiRequestValidator(swagger, &options))

	// Routes
	e.GET("/", hello)

	// Register Handler
	RegisterHandlers(e, &ClinicServer{store: dbstore})

	// Start server
	e.Logger.Print("Starting Server")
	fmt.Printf(ServerString)
	e.Logger.Fatal(e.Start(ServerString))
}

// Handler
func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}


