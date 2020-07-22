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
	if err := c.Store.Find(store.ClinicsPatientsCollection, filter, &pagingParams, &clinicsPatients); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error accessing database")
	}

	return ctx.JSON(http.StatusOK, &clinicsPatients)
}

// AddPatientToClinic
// (POST /clinics/{clinicid}/patients)
func (c *ClinicServer) PostClinicsClinicidPatients(ctx echo.Context, clinicid string) error {
	var clinicsPatients FullClinicsPatients

	if err := ctx.Bind(&clinicsPatients); err != nil {
		log.Printf("Format failed for post clinicsPatients body")
		return echo.NewHTTPError(http.StatusBadRequest, "error parsing parameters")
	}
	clinicsPatients.Active = true
	clinicsPatients.ClinicId = &clinicid

	if newID, err := c.Store.InsertOne(store.ClinicsPatientsCollection, clinicsPatients); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error inserting patient ")
	} else {
		return ctx.JSON(http.StatusOK, map[string]string{"id": *newID})
	}
}

// DeletePatientFromClinic
// (DELETE /clinics/{clinicid}/patients/{patientid})
func (c *ClinicServer) DeleteClinicClinicidPatientsPatientid(ctx echo.Context, clinicid string, patientid string) error {
	filter := bson.M{"clinicId": clinicid, "patientId": patientid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	if err := c.Store.UpdateOne(store.ClinicsPatientsCollection, filter, activeObj); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error deleting patient from clinic")
	}
	return ctx.JSON(http.StatusOK, nil)
}


// GetPatientFromClinic
// (GET /clinics/{clinicid}/patients/{patientid})
func (c *ClinicServer) GetClinicsClinicidPatientsPatientid(ctx echo.Context, clinicid string, patientid string) error {
	var clinicsPatients ClinicsPatients
	log.Printf("Get Clinic by id - id: %s", clinicid)
	filter := bson.M{"clinicId": clinicid, "patientId": patientid, "active": true}
	if err := c.Store.FindOne(store.ClinicsPatientsCollection, filter, &clinicsPatients); err != nil {
		fmt.Println("Find One error ", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error finding patient")
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
	if err := c.Store.UpdateOne(store.ClinicsPatientsCollection, filter, patchObj); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error updating patient")
	}
	return ctx.JSON(http.StatusOK, nil)
}

// Your GET endpoint
// (GET /clinics/patients/{patientid})
func (c *ClinicServer) GetClinicsPatientsPatientid(ctx echo.Context, patientid string) error {

	var clinicsPatients []ClinicsPatients
	log.Printf("Get patient by id - id: %s", patientid)
	filter := bson.M{"patientId": patientid, "active": true}
	pagingParams := store.DefaultPagingParams
	if err := c.Store.Find(store.ClinicsPatientsCollection, filter, &pagingParams, &clinicsPatients); err != nil {
		fmt.Println("Find error ", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error finding patient")
	}

	return ctx.JSON(http.StatusOK, &clinicsPatients)
}

// (DELETE /clinics/patients/{patientid})
func (c *ClinicServer) DeleteClinicsPatientsPatientid(ctx echo.Context, patientid string) error {
	filter := bson.M{ "patientId": patientid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	if err := c.Store.Update(store.ClinicsPatientsCollection, filter, activeObj); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error deleting patient from clinic")
	}
	return ctx.JSON(http.StatusOK, nil)
}