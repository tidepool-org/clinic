package api

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"net/http"
)

type ClinicsCliniciansExtraFields struct {
	Active bool `json:"active" bson:"active"`
}


type FullClinicsClinicians struct {
	ClinicsClinicians `bson:",inline"`
	ClinicsCliniciansExtraFields `bson:",inline"`
}

// GetCliniciansFromClinic
// (GET /clinics/{clinicid}/clinicians)
func (c *ClinicServer) GetClinicsClinicidClinicians(ctx echo.Context, clinicid string, params GetClinicsClinicidCliniciansParams) error {
	filter := bson.M{"clinicId": clinicid, "active": true}

	pagingParams := store.DefaultPagingParams
	if params.Limit != nil {
		pagingParams.Limit = int64(*params.Limit)
	}
	if params.Offset != nil {
		pagingParams.Offset = int64(*params.Offset)
	}

	cursor, err := c.store.Find(store.ClinicsCliniciansCollection, filter, &pagingParams)
	var clinicsClinicians []ClinicsClinicians

	// Probably want to abstract this away in driver
	goctx := context.TODO()
	if err = cursor.All(goctx, &clinicsClinicians); err != nil {
		log.Fatal(err)
	}
	fmt.Println("ret: ", clinicsClinicians)

	ctx.JSON(http.StatusOK, &clinicsClinicians)
	return nil
}

// AddClinicianToClinic
// (POST /clinics/{clinicid}/clinicians)
func (c *ClinicServer) PostClinicsClinicidClinicians(ctx echo.Context, clinicid string) error {
	var clinicsClinicians FullClinicsClinicians
	err := ctx.Bind(&clinicsClinicians)
	clinicsClinicians.Active = true
	clinicsClinicians.ClinicId = &clinicid
	if err != nil {
		log.Printf("Format failed for post clinicsClinicians body")
	}

	c.store.InsertOne(store.ClinicsCliniciansCollection, clinicsClinicians)
	return nil
}

// DeleteClinicianFromClinic
// (DELETE /clinics/{clinicid}/clinicians/{clinicianid})
func (c *ClinicServer) DeleteClinicsClinicidCliniciansClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	filter := bson.M{"clinicId": clinicid, "clinicianId": clinicianid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	c.store.UpdateOne(store.ClinicsCliniciansCollection, filter, activeObj)
	return nil
}

// GetClinician
// (GET /clinics/{clinicid}/clinicians/{clinicianid})
func (c *ClinicServer) GetClinicsClinicidCliniciansClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	var clinicsClinicians ClinicsClinicians
	log.Printf("Get Clinic by id - id: %s", clinicid)
	filter := bson.M{"clinicId": clinicid, "clinicianId": clinicianid, "active": true}
	if err := c.store.FindOne(store.ClinicsCliniciansCollection, filter).Decode(&clinicsClinicians); err != nil {
		fmt.Println("Find One error ", err)
		return nil
	}
	log.Printf("Get Clinic by id - name: %s", clinicsClinicians)

	ctx.JSON(http.StatusOK, &clinicsClinicians)

	return nil
}

// ModifyClinicClinician
// (PATCH /clinics/{clinicid}/clinicians/{clinicianid})
func (c *ClinicServer) PatchClinicsClinicidCliniciansClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	var newClinic ClinicianPermissions
	err := ctx.Bind(&newClinic)
	if err != nil {
		log.Printf("Format failed for patch clinic body")
	}
	filter := bson.M{"clinicId": clinicid, "clinicianId": clinicianid}

	patchObj := bson.D{
		{"$set", newClinic },
	}
	c.store.UpdateOne(store.ClinicsCliniciansCollection, filter, patchObj)
	return nil
}
