package api



import (
	"github.com/getkin/kin-openapi/openapi3filter"
	"context"
	"fmt"
	"errors"
)

// Auth function
func AuthenticationFunc(c context.Context, input *openapi3filter.AuthenticationInput) error {
	fmt.Printf("RVI: %s\n", input.RequestValidationInput.ParamDecoder)
	fmt.Printf("Name: %s\n", input.SecuritySchemeName)
	fmt.Printf("scheme: %s\n", *input.SecurityScheme)
	fmt.Printf("Scopes: %s\n", input.Scopes)

	// Find Role of user
	// Lookup X_TIDEPOOL_USERID in clinic db
	// NOTE - must find clinic id

	// Add tidepool_admin to user roles if exist in header

	// LOOP through scopes
	// If user has the required scope - return

	// No scopes found


	return errors.New("User not authorized for this endpoint")
}
