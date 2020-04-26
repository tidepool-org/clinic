package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"strings"
)

type AuthClient struct {
	store *store.MongoStoreClient
}

var (
	ClinicIdParamName = "clinicid"
	TidepoolUserIdHeaderKey = "X_TIDEPOOL_USERID"
	TidepoolRolesHeaderKey = "X_TIDEPOOL_ROLES"
)


// Auth function
func (a *AuthClient) AuthenticationFunc(c context.Context, input *openapi3filter.AuthenticationInput) error {
	fmt.Printf("Name: %s\n", input.SecuritySchemeName)
	fmt.Printf("scheme: %s\n", *input.SecurityScheme)
	fmt.Printf("Scopes: %s\n", input.Scopes)

	// Find Roles of user
	// Lookup X_TIDEPOOL_USERID in clinic db
	userId := input.RequestValidationInput.Request.Header.Get(TidepoolUserIdHeaderKey)
	fmt.Printf("UserId: %s\n", userId)
	clinicsClinicians, err := a.getUserRoles(&userId)
	if  err != nil {
		fmt.Printf("error getting roles: %s\n", err)

		return err
	}

	// Get the clinic Id
	clinicId := input.RequestValidationInput.PathParams[ClinicIdParamName]


	// Add tidepool_admin to user roles if exist in header
	tidepoolRoles := strings.Split(input.RequestValidationInput.Request.Header.Get(TidepoolRolesHeaderKey), ",")
	roles := tidepoolRoles
	if clinicsClinicians == nil {

		fmt.Printf("Could not retrieve ClinicsClinicians\n")
	} else {
		fmt.Printf("clinicsClinicians: %s\n", *clinicsClinicians)

		// Must belong to clinic
		if clinicId != "" && clinicId != *clinicsClinicians.ClinicId {
			return errors.New("User can not access clinic")
		}
		roles = append(*clinicsClinicians.ClinicianPermissions.Permissions, tidepoolRoles...)
	}
	fmt.Printf("tidepoolRoles: %s\n", tidepoolRoles)
	fmt.Printf("roles: %s\n", roles)

	// LOOP through scopes
	// If user has the required scope - return
	for _, scope := range input.Scopes {
		for _, role := range roles {
			if scope == role {
				fmt.Printf("Matched on %s scope-role", scope)
				return nil
			}
			fmt.Printf("Scope: %s did not match role: %s\n", scope, role)
		}
	}

	// No scopes found
	return errors.New("User not authorized for this endpoint")
}

func (a *AuthClient) getUserRoles(userId *string) (*ClinicsClinicians, error) {
	var clinicsClinicians ClinicsClinicians
	filter := bson.M{"clinicianid": userId, "active": true}
	if err := a.store.FindOne(store.ClinicsCliniciansCollection, filter).Decode(&clinicsClinicians); err != nil {
		// XXX somewhat hacky
		if err == mongo.ErrNoDocuments {
			fmt.Printf("No documents found\n")
			return nil,nil
		}
		return nil, err
	}
	return &clinicsClinicians, nil
}