package api


import (
	"fmt"
	"github.com/tidepool-org/clinic/store"
	"net/http"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	Host = ""
	Port = 3200
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

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(OapiRequestValidator(swagger))

	// Routes
	e.GET("/", hello)

	// Create Store
	e.Logger.Print("Getting Mongog Store")
	dbstore := store.NewMongoStoreClient(mongoHost)

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


