package api

import (
	"context"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3filter"
	"log"
	"net/http"
)

// Create two handler function maps.  One a map of the handler functions for the spec routes.  The other a handler map
// of internal routes
type RouterKey struct {
	Method string
	Path string
}

var (
	InternalRouterMap = make(map[RouterKey]http.HandlerFunc)
	SpecRouterMap = make(map[RouterKey]http.HandlerFunc)

	ReloadMethod = "GET"
	ReloadPath = "/reload"
)


func MainLoop() {
	// Create main router - and load spec file
	router := openapi3filter.NewRouter().WithSwaggerFromFile("clinic.deref.v1.yaml")

	// Reload function - reloads spec file (be nice if we can place it in internal functions in other files)
	InternalRouterMap[RouterKey{ReloadMethod, ReloadPath}] = func (w http.ResponseWriter, r *http.Request) {
		log.Printf("reloading spec file")
		router = openapi3filter.NewRouter().WithSwaggerFromFile("clinic.deref.v1.yaml")
	}

	// Main validation handler
	validationHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set up context
		ctx := context.TODO()
		log.Printf("request.URL is %s, request.Method is %s", r.URL, r.Method)
		// XXX hack for localhost to work
		r.URL.Scheme = "http"
		r.URL.Host = r.Host

		// Check to see if this route exist in our spec
		route, pathParams, err := router.FindRoute(r.Method, r.URL)
		// Validate request
		if (err != nil)  {
			// Route is not in our spec
			// Check to see if in internal spec
			if f, ok := InternalRouterMap[RouterKey{r.Method, r.URL.Path}]; ok {
				// Inside internal spec - use it
				f(w, r)
			} else {
				// Can not find route
				fmt.Fprintf(w, err.Error()+" -- find route error\n")
			}
		} else {

			// Validate request
			requestValidationInput := &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
			}
			if err := openapi3filter.ValidateRequest(ctx, requestValidationInput); err != nil {

				// Request does not pass validation
				w.WriteHeader(400)
				fmt.Fprintf(w, err.Error()+"\n")
			} else {

				// Request validated - check to see if we have a business logic function for route
				fmt.Fprint(w, "{\"version\": \"0.1.0\", \"name\": \"Sample App\"}\n")
				if f, ok := SpecRouterMap[RouterKey{r.Method, r.URL.Path}]; ok {
					f(w, r)
				} else {
					fmt.Fprintf(w, " No functions defined for route\n")

				}
			}
		}
	})


	http.ListenAndServe(":3000", validationHandler)
}
