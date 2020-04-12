package api

import (
	"context"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3filter"
	"io"
	"log"
	"net/http"

)

func mainLoop() {
	//r := mux.NewRouter()
	router := openapi3filter.NewRouter().WithSwaggerFromFile("clinic.deref.v1.yaml")
	// Routes consist of a path and a handler function.
	//r.HandleFunc("/hello", hello)

	validationHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.TODO()
		log.Printf("request.URL is %s", r.URL)
		r.URL.Scheme = "http"
		r.URL.Host = r.Host
		route, pathParams, err := router.FindRoute(r.Method, r.URL)
		// Validate request
		if (err != nil)  {
			io.WriteString(w, err.Error()+" find route error\n")
		}

		requestValidationInput := &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: pathParams,
			Route:      route,
		}
		if err := openapi3filter.ValidateRequest(ctx, requestValidationInput); err != nil {
			w.WriteHeader(400)
			io.WriteString(w, err.Error()+"\n")
		} else {
			fmt.Fprint(w, "{\"version\": \"0.1.0\", \"name\": \"Sample App\"}\n")
		}
	})


	http.ListenAndServe(":3000", validationHandler)
}
