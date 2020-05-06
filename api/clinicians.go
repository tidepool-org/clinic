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
	if err := c.store.Find(store.ClinicsCliniciansCollection, filter, &pagingParams, &clinicsClinicians); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error accessing database")
	}

	return ctx.JSON(http.StatusOK, &clinicsClinicians)
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
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing parameters")
	}

	c.store.InsertOne(store.ClinicsCliniciansCollection, clinicsClinicians)
	return ctx.JSON(http.StatusOK, nil)
}

// DeleteClinicianFromClinic
// (DELETE /clinics/{clinicid}/clinicians/{clinicianid})
func (c *ClinicServer) DeleteClinicsClinicidCliniciansClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	filter := bson.M{"clinicId": clinicid, "clinicianId": clinicianid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	c.store.UpdateOne(store.ClinicsCliniciansCollection, filter, activeObj)
	return ctx.JSON(http.StatusOK, nil)
}

// GetClinician
// (GET /clinics/{clinicid}/clinicians/{clinicianid})
func (c *ClinicServer) GetClinicsClinicidCliniciansClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	var clinicsClinicians ClinicsClinicians
	log.Printf("Get Clinic by id - id: %s", clinicid)
	filter := bson.M{"clinicId": clinicid, "clinicianId": clinicianid, "active": true}
	if err := c.store.FindOne(store.ClinicsCliniciansCollection, filter, &clinicsClinicians); err != nil {
		fmt.Println("Find One error ", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error accessing database")
	}
	//log.Printf("Get Clinic by id - name: %s", clinicsClinicians)

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
	c.store.UpdateOne(store.ClinicsCliniciansCollection, filter, patchObj)
	return ctx.JSON(http.StatusOK, nil)
}
