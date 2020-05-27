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
	"os"
	"strconv"
	"strings"
)

type AuthClient struct {
	store *store.MongoStoreClient
}

var (
	ClinicIdParamName = "clinicid"
	TidepoolUserIdHeaderKey = "X-TIDEPOOL-USERID"
	TidepoolRolesHeaderKey = "X-TIDEPOOL-ROLES"

	KetoMachine = ""
	KetoPort = 0
	DefaultKetoMachine = "localhost"
	DefaultKetoPort = 4456
	KetoUrl = "/engines/acp/ory/glob/allowed"
)

func init() {
	ketoMachine, ok := os.LookupEnv("TIDEPOOL_KETO_HOST")
	if ok {
		KetoMachine = ketoMachine
	} else {
		KetoMachine = DefaultKetoMachine
	}

	ketoPort, ok := os.LookupEnv("TIDEPOOL_KETO_PORT")
	if ok {
		KetoPort, _ = strconv.Atoi(ketoPort)
	} else {
		KetoPort = DefaultKetoPort
	}
}

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
			} else {
				segments = append(segments, "id")
			}
		}
	}
    return strings.Join(segments, ":")

}

// Auth function
func (a *AuthClient) AuthenticationFunc(c context.Context, input *openapi3filter.AuthenticationInput) error {

	//fmt.Printf("%s\n", input.RequestValidationInput.ParamDecoder)
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
		if err == mongo.ErrNoDocuments {
			fmt.Printf("No Documents: %s\n", err)

		} else {
			fmt.Printf("error getting roles: %s\n", err)
			return err

		}
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
		fmt.Printf("error marcshalling json: %s", err)
		return err
	}
	fmt.Printf("req body; %s\n", b)
	hostStr := fmt.Sprintf("http://%s:%d%s", KetoMachine, KetoPort, KetoUrl)
	fmt.Printf("host str; %s\n", hostStr)
	req, err := http.NewRequest("POST", hostStr, bytes.NewBuffer(b))
	if err != nil {
		fmt.Printf("error creating http request: %s", err)
		return err
	}
	fmt.Printf("Finished post creation\n")
	req.Header = headers

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("ERROR contacting keto: %s", err)
		return err
	}
	fmt.Printf("Finished keto - status code: %d\n", resp.StatusCode)
	if resp.StatusCode == 200 {
		return nil
	}
	fmt.Printf("Response: %d", resp.StatusCode)

	// No scopes found
	return errors.New("User not authorized for this endpoint")
}

func (a *AuthClient) getUserRoles(userId *string) (*ClinicsClinicians, error) {
	var clinicsClinicians ClinicsClinicians
	filter := bson.M{"clinicianId": userId, "active": true}
	if err := a.store.FindOne(store.ClinicsCliniciansCollection, filter, &clinicsClinicians); err != nil {
		return nil, err
	}
	return &clinicsClinicians, nil
}