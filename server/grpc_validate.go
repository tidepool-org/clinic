package server

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"google.golang.org/grpc"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
)

const EchoContextKey = "oapi-codegen/echo-context"
const UserDataKey = "oapi-codegen/user-data"

// UnaryServerInterceptor returns a new unary server interceptor that validates incoming messages.
//
// Invalid messages will be rejected with `InvalidArgument` before reaching any userspace handlers.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		fmt.Printf("In validator\n")
		fmt.Printf("Info:  %v\n", info)
		fmt.Printf("Req:  %v\n", req)
		return handler(ctx, req)
	}
}
func OapiRequestValidator(swagger *openapi3.Swagger, options *Options) grpc.UnaryServerInterceptor {
	//router := openapi3filter.NewRouter().WithSwagger(swagger)
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		fmt.Printf("In validator\n")
		return handler(ctx, req)
	}

}

func OapiRequestValidator2(swagger *openapi3.Swagger, options *Options) (func(f http.HandlerFunc) http.HandlerFunc) {
	router := openapi3filter.NewRouter().WithSwagger(swagger)
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Our middleware logic goes here...
			fmt.Printf("In http validator\n")
			err := ValidateRequestFromContext(r, router, options)
			if err != nil {
				fmt.Printf("Error In http validator %s\n", err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// This is an Echo middleware function which validates incoming HTTP requests
// to make sure that they conform to the given OAPI 3.0 specification. When
// OAPI validation failes on the request, we return an HTTP/400.

// Options to customize request validation. These are passed through to
// openapi3filter.
type Options struct {
	Options      openapi3filter.Options
	ParamDecoder openapi3filter.ContentParameterDecoder
	UserData     interface{}
}

// Create a validator from a swagger object, with validation options
//func OapiRequestValidator(swagger *openapi3.Swagger, options *Options) echo.MiddlewareFunc {
//	router := openapi3filter.NewRouter().WithSwagger(swagger)
//	return func(next echo.HandlerFunc) echo.HandlerFunc {
//		return func(c echo.Context) error {
//			err := ValidateRequestFromContext(c, router, options)
//			if err != nil {
//				return err
//			}
//			return next(c)
//		}
//	}
//}

// This function is called from the middleware above and actually does the work
// of validating a request.
//func ValidateRequestFromContext(ctx echo.Context, router *openapi3filter.Router, options *Options) error {
func ValidateRequestFromContext(req *http.Request, router *openapi3filter.Router, options *Options) error {
	// XXX hack to make localhost work
	fmt.Printf("Scheme: %s, Host: %s\n", req.URL.Scheme, req.URL.Host)
	fmt.Printf("Headers: %s\n", req.Header)
	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	fmt.Printf("Fixed Scheme: %s, Host: %s\n", req.URL.Scheme, req.URL.Host)
	route, pathParams, err := router.FindRoute(req.Method, req.URL)

	// We failed to find a matching route for the request.
	if err != nil {
		switch e := err.(type) {
		case *openapi3filter.RouteError:
			// We've got a bad request, the path requested doesn't match
			// either server, or path, or something.
			return echo.NewHTTPError(http.StatusBadRequest, e.Reason)
		default:
			// This should never happen today, but if our upstream code changes,
			// we don't want to crash the server, so handle the unexpected error.
			return echo.NewHTTPError(http.StatusInternalServerError,
				fmt.Sprintf("error validating route: %s", err.Error()))
		}
	}

	validationInput := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
	}

	// Pass the Echo context into the request validator, so that any callbacks
	// which it invokes make it available.
	//requestContext := context.WithValue(context.Background(), EchoContextKey, ctx)
	requestContext := context.Background()

	if options != nil {
		validationInput.Options = &options.Options
		validationInput.ParamDecoder = options.ParamDecoder
		requestContext = context.WithValue(requestContext, UserDataKey, options.UserData)

		if options.Options.AuthenticationFunc != nil {
			if err := options.Options.AuthenticationFunc(requestContext, &openapi3filter.AuthenticationInput{
				RequestValidationInput: validationInput,
			}); err != nil {
				return err
			}
		}
	}

	err = openapi3filter.ValidateRequest(requestContext, validationInput)
	if err != nil {
		switch e := err.(type) {
		case *openapi3filter.RequestError:
			// We've got a bad request
			// Split up the verbose error by lines and return the first one
			// openapi errors seem to be multi-line with a decent message on the first
			errorLines := strings.Split(e.Error(), "\n")
			return &echo.HTTPError{
				Code:     http.StatusBadRequest,
				Message:  errorLines[0],
				Internal: err,
			}
		case *openapi3filter.SecurityRequirementsError:
			for _, err := range e.Errors {
				httpErr, ok := err.(*echo.HTTPError)
				if ok {
					return httpErr
				}
			}
			return &echo.HTTPError{
				Code:     http.StatusForbidden,
				Message:  e.Error(),
				Internal: err,
			}
		default:
			// This should never happen today, but if our upstream code changes,
			// we don't want to crash the server, so handle the unexpected error.
			return &echo.HTTPError{
				Code:     http.StatusInternalServerError,
				Message:  fmt.Sprintf("error validating request: %s", err),
				Internal: err,
			}
		}
	}
	return nil
}

