package api

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"net/http"
	"os"
)

type ClinicServer struct {
	store *store.MongoStoreClient
}

// (GET /clinic)
func (c *ClinicServer) GetClinic(ctx echo.Context) error {

	log.Printf("Get Clinic - name")
	return nil
}

// createClinic
// (POST /clinic)
func (c *ClinicServer) PostClinic(ctx echo.Context) error {
	var newClinic NewClinic
	err := ctx.Bind(&newClinic)
	if err != nil {
		log.Printf("Format failed for clinic body")
	}

	log.Printf("Clinic address: %s", *newClinic.Address)
	c.store.InsertOne(newClinic)
	return nil
}

// (DELETE /clinic/{clinicid})
func (c *ClinicServer) DeleteClinicClinicid(ctx echo.Context, clinicid string) error {
	return nil
}
// getClinic
// (GET /clinic/{clinicid})
func (c *ClinicServer) GetClinicClinicid(ctx echo.Context, clinicid string) error {
	var clinic Clinic
	log.Printf("Get Clinic by id - id: %s", clinicid)
	if err := c.store.FindOne(bson.M{"_id": clinicid}).Decode(&clinic); err != nil {
		fmt.Println("Find One error ", err)
		os.Exit(1)
	}
	log.Printf("test")
	//log.Printf("Get Clinic by id - name: %s, id: %s", *newClinic.Name, *newClinic.Id)
	log.Printf("Get Clinic by id - name: %s", clinic)

	ctx.JSON(http.StatusOK, &clinic)

	return nil
}

// (PATCH /clinic/{clinicid})
func (c *ClinicServer) PatchClinicClinicid(ctx echo.Context, clinicid string) error {
	return nil
}
// DeleteClinicianForClinic
// (DELETE /clinic/{clinicid}/clinician/{clinicianid})
func (c *ClinicServer) DeleteClinicClinicidClinicianClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	return nil
}

// (PATCH /clinic/{clinicid}/clinician/{clinicianid})
func (c *ClinicServer) PatchClinicClinicidClinicianClinicianid(ctx echo.Context, clinicid string, clinicianid string) error {
	return nil
}
// deletePatientFromClinic
// (DELETE /clinic/{clinicid}/patient/{patientid})
func (c *ClinicServer) DeleteClinicClinicidPatientPatientid(ctx echo.Context, clinicid string, patientid string) error {
	return nil
}
// addPatientToClinic
// (PATCH /clinic/{clinicid}/patient/{patientid})
func (c *ClinicServer) PatchClinicClinicidPatientPatientid(ctx echo.Context, clinicid string, patientid string) error {
	return nil
}

// (POST /clinic/{clinicid}/patient/{patientid})
func (c *ClinicServer) PostClinicClinicidPatientPatientid(ctx echo.Context, clinicid string, patientid string) error {
	return nil
}
// getCliniciansForClinic
// (GET /clinic/{clinicid}/patients)
func (c *ClinicServer) GetClinicClinicidPatients(ctx echo.Context, clinicid string) error {
	return nil
}
// getClinic
// (GET /clinics)
func (c *ClinicServer) GetClinics(ctx echo.Context) error {
	return nil
}
// getCliniciansForClinic
// (GET /clinics/{clinicid}/clinicians)
func (c *ClinicServer) GetClinicsClinicidClinicians(ctx echo.Context, clinicid string) error {
	return nil
}
