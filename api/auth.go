package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"strings"
)

type AuthClient struct {
	store *store.MongoStoreClient
}

var (
	ClinicIdParamName = "clinicid"
	TidepoolUserIdHeaderKey = "X_TIDEPOOL_USERID"
	TidepoolRolesHeaderKey = "X_TIDEPOOL_ROLES"

	KetoMachine = "localhost"
	KetoPort = 4456
	KetoUrl = "/engines/acp/ory/glob/allowed"
)

type ContextRec struct {
	Role string `json:"role"`
	Clinics [][]string  `json:"clinics"`
}

type KetoRequest struct {
	Subject string `json:"subject"`
	Action string `json:"action"`
	Resource string `json:"resource"`
	Context ContextRec  `json:"context"`
}

func getResourceFromPath(path *string) string {
    segments := make([]string, 0)
    segments = append(segments, "resources")
	if path != nil {
		pathArray := strings.Split(strings.TrimLeft(*path, "/"), "/")
		for _, path := range pathArray {
			if strings.HasPrefix(path, "{") != true {
				segments = append(segments, path)
			}
		}
	}
    return strings.Join(segments, ":")

}

// Auth function
func (a *AuthClient) AuthenticationFunc(c context.Context, input *openapi3filter.AuthenticationInput) error {

	fmt.Printf("%s\n", input.RequestValidationInput.ParamDecoder)
	// Find Roles of user
	// Lookup X_TIDEPOOL_USERID in clinic db
	userId := input.RequestValidationInput.Request.Header.Get(TidepoolUserIdHeaderKey)
	path := input.RequestValidationInput.Route.Path
	method := strings.ToLower(input.RequestValidationInput.Request.Method)
	fmt.Printf("Path: %s\n", getResourceFromPath(&path))
	fmt.Printf("Method: %s\n", method)
	fmt.Printf("UserId: %s\n", userId)
	clinicsClinicians, err := a.getUserRoles(&userId)
	if  err != nil {
		fmt.Printf("error getting roles: %s\n", err)

		return err
	}

	// Get the clinic Id
	clinicId := input.RequestValidationInput.PathParams[ClinicIdParamName]


	// Add tidepool_admin to user roles if exist in header
	tidepoolRolesStr := input.RequestValidationInput.Request.Header.Get(TidepoolRolesHeaderKey)
	var tidepoolRoles []string
	if tidepoolRolesStr != "" {
		tidepoolRoles = strings.Split(tidepoolRolesStr, ",")
	}
	roles := tidepoolRoles
	dbClinicId := ""
	if clinicsClinicians == nil {

		fmt.Printf("Could not retrieve ClinicsClinicians\n")
	} else {
		//fmt.Printf("clinicsClinicians: %s\n", *clinicsClinicians)

		// Must belong to clinic
		//if clinicId != "" && clinicId != *clinicsClinicians.ClinicId {
		//	return errors.New("User can not access clinic")
		//}
		dbClinicId = *clinicsClinicians.ClinicId
		roles = append(*clinicsClinicians.ClinicianPermissions.Permissions, tidepoolRoles...)
	}
	var rolesArray []string
	for _, role := range roles {
		rolesArray = append(rolesArray, fmt.Sprintf("{%s}", role))
	}
	fmt.Printf("tidepoolRoles: %s,  len: %d\n", tidepoolRoles, len(tidepoolRoles))
	fmt.Printf("roles: %s,  len: %d,  str: %s\n", roles, len(roles), strings.Join(rolesArray, ","))



	headers := map[string][]string{
		"Content-Type": []string{"application/json"},
		"Accept": []string{"application/json"},
	}

	ketoReq := KetoRequest{
		Subject: fmt.Sprintf("users:%s", userId),
		Action:   method,
		Resource: getResourceFromPath(&path),
		Context: ContextRec{
			Role:    strings.Join(rolesArray, ","),
			Clinics: [][]string{{clinicId, dbClinicId}},
		},
	}

	b, err := json.Marshal(ketoReq)
	if err != nil {
		fmt.Printf("error marcshalling json: ", err)
		return err
	}
	fmt.Printf("req body; %s", b)
	hostStr := fmt.Sprintf("http://%s:%d%s", KetoMachine, KetoPort, KetoUrl)
	req, err := http.NewRequest("POST", hostStr, bytes.NewBuffer(b))
	if err != nil {
		fmt.Printf("error creating http request: ", err)
		return err
	}
	req.Header = headers

	client := &http.Client{}
	if err != nil {
		fmt.Printf("error contacting keto: ", err)
		return err
	}
	resp, err := client.Do(req)
	if resp.StatusCode == 200 {
		return nil
	}
	// If user has the required scope - return
	//for _, scope := range input.Scopes {
	//	for _, role := range roles {
	//		if scope == role {
	//			fmt.Printf("Matched on %s scope-role", scope)
	//			return nil
	//		}
	//		fmt.Printf("Scope: %s did not match role: %s\n", scope, role)
	//	}
	//}

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