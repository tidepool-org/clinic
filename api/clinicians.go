package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
	"fmt"
	"os"
	"context"
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
	clinicsClinician := ClinicsClinicians{ClinicId: &clinicid}
	filter := FullClinicsClinicians{ClinicsCliniciansExtraFields: ClinicsCliniciansExtraFields{Active: true}, 
		                            ClinicsClinicians: clinicsClinician}
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

	ctx.JSON(http.StatusOK, &clinicsClinicians)
	return nil
}

// AddClinicianToClinic
// (POST /clinics/{clinicid}/clinicians)
func (c *ClinicServer) PostClinicsClinicidClinicians(ctx echo.Context, clinicid string) error {
	var clinicsClinicians FullClinicsClinicians
	err := ctx.Bind(&clinicsClinicians)
	clinicsClinicians.Active = true
	if err != nil {
		log.Printf("Format failed for post clinicsClinicians body")
	}

	c.store.InsertOne(store.ClinicsCliniciansCollection, clinicsClinicians)
	return nil
}

// DeleteClinicianFromClinic
// (DELETE /clinics/{clinicid}/clinicians/{clinicianid})
func (c *ClinicServer) DeleteClinicsClinicidCliniciansClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	clinicObjID, _ := primitive.ObjectIDFromHex(clinicid)
	clinicianObjID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.M{"ClinicId": clinicObjID, "ClinicianId": clinicianObjID}
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
	clinicObjID, _ := primitive.ObjectIDFromHex(clinicid)
	clinicianObjID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.M{"ClinicId": clinicObjID, "ClinicianId": clinicianObjID, "active": true}
	if err := c.store.FindOne(store.ClinicsCliniciansCollection, filter).Decode(&clinicsClinicians); err != nil {
		fmt.Println("Find One error ", err)
		os.Exit(1)
	}
	log.Printf("test")
	//log.Printf("Get Clinic by id - name: %s, id: %s", *newClinic.Name, *newClinic.Id)
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
	clinicObjID, _ := primitive.ObjectIDFromHex(clinicid)
	clinicianObjID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.M{"ClinicId": clinicObjID, "ClinicianId": clinicianObjID}

	patchObj := bson.D{
		{"$set", newClinic },
	}
	c.store.UpdateOne(store.ClinicsCliniciansCollection, filter, patchObj)
	return nil
}
