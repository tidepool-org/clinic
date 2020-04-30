package api

import (
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"net/http"
	"fmt"
	"os"
	"context"
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

	cursor, err := c.store.Find(store.ClinicsPatientsCollection, filter, &pagingParams)
	var clinicsPatients []ClinicsPatients

	// Probably want to abstract this away in driver
	goctx := context.TODO()
	if err = cursor.All(goctx, &clinicsPatients); err != nil {
		log.Fatal(err)
	}
	fmt.Println("ret: ", clinicsPatients)

	ctx.JSON(http.StatusOK, &clinicsPatients)
	return nil
}

// AddPatientToClinic
// (POST /clinics/{clinicid}/patients)
func (c *ClinicServer) PostClinicsClinicidPatients(ctx echo.Context, clinicid string) error {
	var clinicsPatients FullClinicsPatients
	err := ctx.Bind(&clinicsPatients)
	clinicsPatients.Active = true
	clinicsPatients.ClinicId = &clinicid
	if err != nil {
		log.Printf("Format failed for post clinicsPatients body")
	}

	c.store.InsertOne(store.ClinicsPatientsCollection, clinicsPatients)
	return nil
}

// DeletePatientFromClinic
// (DELETE /clinics/{clinicid}/patients/{patientid})
func (c *ClinicServer) DeleteClinicClinicidPatientsPatientid(ctx echo.Context, clinicid string, patientid string) error {
	filter := bson.M{"clinicId": clinicid, "patientId": patientid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	c.store.UpdateOne(store.ClinicsPatientsCollection, filter, activeObj)
	return nil
}


// GetPatientFromClinic
// (GET /clinics/{clinicid}/patients/{patientid})
func (c *ClinicServer) GetClinicsClinicidPatientsPatientid(ctx echo.Context, clinicid string, patientid string) error {
	var clinicsPatients ClinicsPatients
	log.Printf("Get Clinic by id - id: %s", clinicid)
	filter := bson.M{"clinicId": clinicid, "patientId": patientid, "active": true}
	if err := c.store.FindOne(store.ClinicsPatientsCollection, filter).Decode(&clinicsPatients); err != nil {
		fmt.Println("Find One error ", err)
		os.Exit(1)
	}
	//log.Printf("Get Clinic by id - name: %s", clinicsPatients)

	ctx.JSON(http.StatusOK, &clinicsPatients)
	return nil
}

// ModifyClinicPatient
// (PATCH /clinics/{clinicid}/patients/{patientid})
func (c *ClinicServer) PatchClinicsClinicidPatientsPatientid(ctx echo.Context, clinicid string, patientid string) error {
	var newPatient PatientPermissions
	err := ctx.Bind(&newPatient)
	if err != nil {
		log.Printf("Format failed for patch clinic body")
	}
	filter := bson.M{"clinicId": clinicid, "patientId": patientid}

	patchObj := bson.D{
		{"$set", newPatient },
	}
	c.store.UpdateOne(store.ClinicsPatientsCollection, filter, patchObj)
	return nil
}