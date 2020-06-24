package api

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net/http"
)

var (
	CLINIC_ADMIN = "CLINIC_ADMIN"
)

type ClinicServer struct {
	Store store.StorageInterface
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

type PostID struct {
	Id string `json:"id,omitempty" bson:"id,omitempty"`
}
// getClinic
// (GET /clinics)
func (c *ClinicServer) GetClinics(ctx echo.Context, params GetClinicsParams) error {
	newClinic := NewClinic{}
	if params.ClinicianId != nil  && params.PatientId != nil {
		// This is an error - can not search by clinician and patient
		return echo.NewHTTPError(http.StatusBadRequest, "Can not filter by both clinicianId and patientId ")
	}

	// XXX Auth on this one needs to be thought through
	// XXX Empty result returns an error
	// This part is a little funky - we are going to search in the specific collection for patient or clinician
	// vs doing a join.  Main reason is that joins are not supported in mongo between strings and objectIds.  Should probably
	// store clinics as objectIds
	var filter interface{}
	if params.ClinicianId != nil {
		clinicianFilter := bson.M{"clinicianId": params.ClinicianId, "active": true}
		var clinicsClinicians ClinicsClinicians
		if err := c.Store.FindOne(store.ClinicsCliniciansCollection, clinicianFilter, &clinicsClinicians); err != nil {
			fmt.Println("Find One error ", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "error accessing database")
		}
		objID, _ := primitive.ObjectIDFromHex(*clinicsClinicians.ClinicId)
		filter = bson.M{"_id": objID}

	} else if params.PatientId != nil {
		patientFilter := bson.M{"clinicianId": params.PatientId, "active": true}
		var clinicsPatients ClinicsPatients
		if err := c.Store.FindOne(store.ClinicsPatientsCollection, patientFilter, &clinicsPatients); err != nil {
			fmt.Println("Find One error ", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "error accessing database")
		}
		objID, _ := primitive.ObjectIDFromHex(*clinicsPatients.ClinicId)
		filter = bson.M{"_id": objID}

	} else {

		filter = FullNewClinic{ClinicExtraFields: ClinicExtraFields{Active: true}, NewClinic: newClinic}
	}



	// Get Paging params
	pagingParams := store.DefaultPagingParams
	if params.Limit != nil {
		pagingParams.Limit = int64(*params.Limit)
	}
	if params.Offset != nil {
		pagingParams.Offset = int64(*params.Offset)
	}

	var clinics []Clinic
	if err := c.Store.Find(store.ClinicsCollection, filter, &pagingParams, &clinics); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error finding clinic")
	}

	return ctx.JSON(http.StatusOK, &clinics)
}

// createClinic
// (POST /clinics)
func (c *ClinicServer) PostClinics(ctx echo.Context) error {
	var newClinic FullNewClinic

	if err := ctx.Bind(&newClinic); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing parameters")
	}
	newClinic.Active = true

	var clinicsClinicians FullClinicsClinicians
	if newID, err := c.Store.InsertOne(store.ClinicsCollection, newClinic); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error inserting clinic")
	} else {
		clinicsClinicians.ClinicId = newID
	}

	// We also have to add an initial admin - the user
	userId := ctx.Request().Header.Get(TidepoolUserIdHeaderKey)
	clinicsClinicians.Active = true
	clinicsClinicians.ClinicianId = userId
	clinicsClinicians.Permissions = &[]string{CLINIC_ADMIN}
	if _, err := c.Store.InsertOne(store.ClinicsCliniciansCollection, clinicsClinicians); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error inserting to clinician")
	} else {
		postID := PostID{Id: *clinicsClinicians.ClinicId}
		//postID := PostID{newid: *newID, test: "test"}
		log.Printf("Returning from newer /clinics, %s, %s", postID.Id, *clinicsClinicians.ClinicId)
		return ctx.JSON(http.StatusOK, &postID)
	}
}

// (DELETE /clinic/{clinicid})
func (c *ClinicServer) DeleteClinicsClinicid(ctx echo.Context, clinicid string) error {
	objID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.D{{"_id", objID}}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	if err := c.Store.UpdateOne(store.ClinicsCollection, filter, activeObj); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error deleting clinic from database")
	}
	return ctx.JSON(http.StatusOK, nil)
}

// getClinic
// (GET /clinic/{clinicid})
func (c *ClinicServer) GetClinicsClinicid(ctx echo.Context, clinicid string) error {
	var clinic Clinic
	log.Printf("Get Clinic by id - id: %s", clinicid)
	objID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.M{"_id": objID, "active": true}
	if err := c.Store.FindOne(store.ClinicsCollection, filter, &clinic); err != nil {
		fmt.Println("Find One error ", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error finding clinic")
	}

	return ctx.JSON(http.StatusOK, &clinic)
}

// (PATCH /clinic/{clinicid})
func (c *ClinicServer) PatchClinicsClinicid(ctx echo.Context, clinicid string) error {
	var newClinic NewClinic

	if err := ctx.Bind(&newClinic); err != nil {
		log.Printf("Format failed for patch clinic body")
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing parameters")
	}
	objID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.D{{"_id", objID}}
	patchObj := bson.D{
		{"$set", newClinic },
	}
	if err := c.Store.UpdateOne(store.ClinicsCollection, filter, patchObj); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error updating clinic")
	}
	return ctx.JSON(http.StatusOK, nil)
}

