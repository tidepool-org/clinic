package api

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"net/http"
)

type ClinicsPatientsExtraFields struct {
	Active bool `json:"active" bson:"active"`
}


type FullClinicsPatients struct {
	ClinicsPatients `bson:",inline"`
	ClinicsPatientsExtraFields `bson:",inline"`
}

// GetPatientsForClinic
// (GET /clinics/{clinicid}/patients)
func (c *ClinicServer) GetClinicsClinicidPatients(ctx echo.Context, clinicid string, params GetClinicsClinicidPatientsParams) error {
	filter := bson.M{"clinicId": clinicid, "active": true}

	pagingParams := store.DefaultPagingParams
	if params.Limit != nil {
		pagingParams.Limit = int64(*params.Limit)
	}
	if params.Offset != nil {
		pagingParams.Offset = int64(*params.Offset)
	}

	var clinicsPatients []ClinicsPatients
	if err := c.store.Find(store.ClinicsPatientsCollection, filter, &pagingParams, &clinicsPatients); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error accessing database")
	}

	return ctx.JSON(http.StatusOK, &clinicsPatients)
}

// AddPatientToClinic
// (POST /clinics/{clinicid}/patients)
func (c *ClinicServer) PostClinicsClinicidPatients(ctx echo.Context, clinicid string) error {
	var clinicsPatients FullClinicsPatients
	err := ctx.Bind(&clinicsPatients)
	if err != nil {
		log.Printf("Format failed for post clinicsPatients body")
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing parameters")
	}
	clinicsPatients.Active = true
	clinicsPatients.ClinicId = &clinicid

	c.store.InsertOne(store.ClinicsPatientsCollection, clinicsPatients)
	return ctx.JSON(http.StatusOK, nil)
}

// DeletePatientFromClinic
// (DELETE /clinics/{clinicid}/patients/{patientid})
func (c *ClinicServer) DeleteClinicClinicidPatientsPatientid(ctx echo.Context, clinicid string, patientid string) error {
	filter := bson.M{"clinicId": clinicid, "patientId": patientid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	c.store.UpdateOne(store.ClinicsPatientsCollection, filter, activeObj)
	return ctx.JSON(http.StatusOK, nil)
}


// GetPatientFromClinic
// (GET /clinics/{clinicid}/patients/{patientid})
func (c *ClinicServer) GetClinicsClinicidPatientsPatientid(ctx echo.Context, clinicid string, patientid string) error {
	var clinicsPatients ClinicsPatients
	log.Printf("Get Clinic by id - id: %s", clinicid)
	filter := bson.M{"clinicId": clinicid, "patientId": patientid, "active": true}
	if err := c.store.FindOne(store.ClinicsPatientsCollection, filter, &clinicsPatients); err != nil {
		fmt.Println("Find One error ", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error accessing database")
	}

	return ctx.JSON(http.StatusOK, &clinicsPatients)
}

// ModifyClinicPatient
// (PATCH /clinics/{clinicid}/patients/{patientid})
func (c *ClinicServer) PatchClinicsClinicidPatientsPatientid(ctx echo.Context, clinicid string, patientid string) error {
	var newPatient PatientPermissions
	err := ctx.Bind(&newPatient)
	if err != nil {
		log.Printf("Format failed for patch clinic body")
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing parameters")
	}
	filter := bson.M{"clinicId": clinicid, "patientId": patientid}

	patchObj := bson.D{
		{"$set", newPatient },
	}
	c.store.UpdateOne(store.ClinicsPatientsCollection, filter, patchObj)
	return ctx.JSON(http.StatusOK, nil)
}