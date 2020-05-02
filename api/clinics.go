package api

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
	"os"
	"context"
)

type ClinicServer struct {
	store *store.MongoStoreClient
}

type ClinicExtraFields struct {
	Active bool   `json:"active" bson:"active"`
}

type FullClinic struct {
	Clinic `bson:",inline"`
	ClinicExtraFields `bson:",inline"`
}

type FullNewClinic struct {
	NewClinic `bson:",inline"`
	ClinicExtraFields `bson:",inline"`
}
// getClinic
// (GET /clinics)
func (c *ClinicServer) GetClinics(ctx echo.Context, params GetClinicsParams) error {
	newClinic := NewClinic{}
	filter := FullNewClinic{ClinicExtraFields: ClinicExtraFields{Active: true}, NewClinic: newClinic}
	pagingParams := store.DefaultPagingParams
	if params.Limit != nil {
		pagingParams.Limit = int64(*params.Limit)
	}
	if params.Offset != nil {
		pagingParams.Offset = int64(*params.Offset)
	}

	cursor, err := c.store.Find(store.ClinicsCollection, filter, &pagingParams)
	var clinics []Clinic

	// Probably want to abstract this away in driver
	goctx := context.TODO()
	if err = cursor.All(goctx, &clinics); err != nil {
		log.Fatal(err)
	}

	ctx.JSON(http.StatusOK, &clinics)

	return nil
}

// createClinic
// (POST /clinics)
func (c *ClinicServer) PostClinics(ctx echo.Context) error {
	var newClinic FullNewClinic
	err := ctx.Bind(&newClinic)
	if err != nil {
		log.Printf("Format failed for post clinic body")
	}
	newClinic.Active = true

	log.Printf("Clinic address: %s", *newClinic.Address)
	result, _ := c.store.InsertOne(store.ClinicsCollection, newClinic)

	// We also have to add an initial admin - the user
	userId := ctx.Request().Header.Get(TidepoolUserIdHeaderKey)
	var clinicsClinicians FullClinicsClinicians
	clinicsClinicians.Active = true
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		newID := oid.Hex()
		clinicsClinicians.ClinicId = &newID
	}
	clinicsClinicians.ClinicianId = userId
	clinicsClinicians.Permissions = &[]string{"CLINIC_ADMIN"}
	c.store.InsertOne(store.ClinicsCliniciansCollection, clinicsClinicians)

	return nil
}

// (DELETE /clinic/{clinicid})
func (c *ClinicServer) DeleteClinicsClinicid(ctx echo.Context, clinicid string) error {
	objID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.D{{"_id", objID}}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	c.store.UpdateOne(store.ClinicsCollection, filter, activeObj)
	return nil
}

// getClinic
// (GET /clinic/{clinicid})
func (c *ClinicServer) GetClinicsClinicid(ctx echo.Context, clinicid string) error {
	var clinic Clinic
	log.Printf("Get Clinic by id - id: %s", clinicid)
	objID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.M{"_id": objID, "active": true}
	if err := c.store.FindOne(store.ClinicsCollection, filter).Decode(&clinic); err != nil {
		fmt.Println("Find One error ", err)
		os.Exit(1)
	}
	log.Printf("test")
	//log.Printf("Get Clinic by id - name: %s, id: %s", *newClinic.Name, *newClinic.Id)
	//log.Printf("Get Clinic by id - name: %s", clinic)

	ctx.JSON(http.StatusOK, &clinic)

	return nil
}

// (PATCH /clinic/{clinicid})
func (c *ClinicServer) PatchClinicsClinicid(ctx echo.Context, clinicid string) error {
	var newClinic NewClinic
	err := ctx.Bind(&newClinic)
	if err != nil {
		log.Printf("Format failed for patch clinic body")
	}
	objID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.D{{"_id", objID}}
	patchObj := bson.D{
		{"$set", newClinic },
	}
	c.store.UpdateOne(store.ClinicsCollection, filter, patchObj)
	return nil
}

