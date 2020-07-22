package api

import (
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

	var clinicsClinicians []ClinicsClinicians
	if err := c.Store.Find(store.ClinicsCliniciansCollection, filter, &pagingParams, &clinicsClinicians); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error finding clinician")
	}

	return ctx.JSON(http.StatusOK, &clinicsClinicians)
}

// AddClinicianToClinic
// (POST /clinics/{clinicid}/clinicians)
func (c *ClinicServer) PostClinicsClinicidClinicians(ctx echo.Context, clinicid string) error {
	var clinicsClinicians FullClinicsClinicians

	if err := ctx.Bind(&clinicsClinicians); err != nil {
		log.Printf("Format failed for post clinicsClinicians body")
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing parameters")
	}
	clinicsClinicians.Active = true
	clinicsClinicians.ClinicId = &clinicid

	if newID, err := c.Store.InsertOne(store.ClinicsCliniciansCollection, clinicsClinicians); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error inserting clinician")
	} else {
		return ctx.JSON(http.StatusOK, map[string]string{"id": *newID})
	}
}

// DeleteClinicianFromClinic
// (DELETE /clinics/{clinicid}/clinicians/{clinicianid})
func (c *ClinicServer) DeleteClinicsClinicidCliniciansClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	// If a clinic administrator - we must ensure that we have one clinic admin object

	// We are a bit clever in determining this - we first try to find 2 admins.  If only one and that one is the
	// one that we are attempting to delete - return an error
	var clinicsClinicians []ClinicsClinicians
	filter := bson.M{"clinicId": clinicid, "active": true, "permissions": CLINIC_ADMIN}
	pagingParams := store.DefaultPagingParams
	pagingParams.Limit = 2
	if err := c.Store.Find(store.ClinicsCliniciansCollection, filter, &pagingParams, &clinicsClinicians); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error accessing clinic database")
	}
	if len(clinicsClinicians) == 1 && clinicsClinicians[0].ClinicianId == clinicianid {
		return echo.NewHTTPError(http.StatusBadRequest, "Can not delete last clinic administrator")
	}

	// Passed check - Now delete clinician
	filter = bson.M{"clinicId": clinicid, "clinicianId": clinicianid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	if err := c.Store.UpdateOne(store.ClinicsCliniciansCollection, filter, activeObj); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error deleting clinician from database")
	}
	return ctx.JSON(http.StatusOK, nil)
}

// GetClinician
// (GET /clinics/{clinicid}/clinicians/{clinicianid})
func (c *ClinicServer) GetClinicsClinicidCliniciansClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	var clinicsClinicians ClinicsClinicians
	log.Printf("Get Clinic by id - id: %s", clinicid)
	filter := bson.M{"clinicId": clinicid, "clinicianId": clinicianid, "active": true}
	if err := c.Store.FindOne(store.ClinicsCliniciansCollection, filter, &clinicsClinicians); err != nil {
		fmt.Println("Find One error ", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error accessing database")
	}

	return ctx.JSON(http.StatusOK, &clinicsClinicians)
}

// ModifyClinicClinician
// (PATCH /clinics/{clinicid}/clinicians/{clinicianid})
func (c *ClinicServer) PatchClinicsClinicidCliniciansClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	var newClinic ClinicianPermissions

	err := ctx.Bind(&newClinic)
	if err != nil {
		log.Printf("Format failed for patch clinic body")
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing parameters")
	}
	filter := bson.M{"clinicId": clinicid, "clinicianId": clinicianid}

	patchObj := bson.D{
		{"$set", newClinic },
	}
	if err := c.Store.UpdateOne(store.ClinicsCliniciansCollection, filter, patchObj); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error updating clinician")
	}
	return ctx.JSON(http.StatusOK, nil)
}


// Your GET endpoint
// (GET /clinics/clinicians/{clinicianid})
func (c *ClinicServer) GetClinicsCliniciansClinicianid(ctx echo.Context, clinicianid string) error {
	var clinicsClinicians []ClinicsClinicians
	log.Printf("Get patient by id - id: %s", clinicianid)
	filter := bson.M{"clinicianId": clinicianid, "active": true}
	pagingParams := store.DefaultPagingParams
	if err := c.Store.Find(store.ClinicsCliniciansCollection, filter, &pagingParams, &clinicsClinicians); err != nil {
		fmt.Println("Find error ", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error finding clinician")
	}

	return ctx.JSON(http.StatusOK, &clinicsClinicians)
}

// (DELETE /clinics/clinicians/{clinicianid})
func (c *ClinicServer) DeleteClinicsCliniciansClinicianid(ctx echo.Context, clinicianid string) error {
	filter := bson.M{ "clinicianId": clinicianid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	if err := c.Store.Update(store.ClinicsCliniciansCollection, filter, activeObj); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error deleting clinician from clinic")
	}
	return ctx.JSON(http.StatusOK, nil)

}
