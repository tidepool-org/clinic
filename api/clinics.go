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
	PATIENT_ALL_PAGING_LIMIT int64 = 100000
	CLINICIAN_ALL_PAGING_LIMIT int64 = 100000
	CLINICS_ALL_PAGING_LIMIT int64 = 100000
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

	// Get Paging params
	pagingParams := store.DefaultPagingParams
	if params.Limit != nil {
		pagingParams.Limit = int64(*params.Limit)
	}
	if params.Offset != nil {
		pagingParams.Offset = int64(*params.Offset)
	}

	// XXX Auth on this one needs to be thought through
	// XXX Empty result returns an error
	// This part is a little funky - we are going to search in the specific collection for patient or clinician
	// vs doing a join.  Main reason is that joins are not supported in mongo between strings and objectIds.  Should probably
	// store clinics as objectIds
	var filter interface{}
	if params.ClinicianId != nil {
		clinicianFilter := bson.M{"clinicianId": params.ClinicianId, "active": true}
		var clinicsClinicians[] ClinicsClinicians
		if err := c.Store.Find(store.ClinicsCliniciansCollection, clinicianFilter, &pagingParams, &clinicsClinicians); err != nil {
			fmt.Println("Find One error ", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "error accessing database")
		}
		var objIDArray[] primitive.ObjectID
		for _, clinicsClinician := range(clinicsClinicians) {

			objID, _ := primitive.ObjectIDFromHex(*clinicsClinician.ClinicId)
			objIDArray = append(objIDArray, objID)
		}
		filter = bson.M{"_id": bson.M{"$in": objIDArray}}

	} else if params.PatientId != nil {
		patientFilter := bson.M{"patientId": params.PatientId}
		var clinicsPatients[] ClinicsPatients
		if err := c.Store.Find(store.ClinicsPatientsCollection, patientFilter, &pagingParams, &clinicsPatients); err != nil {
			fmt.Println("Find One error ", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "error accessing database")
		}
		var objIDArray[] primitive.ObjectID
		for _, clinicsPatient := range(clinicsPatients) {

			objID, _ := primitive.ObjectIDFromHex(*clinicsPatient.ClinicId)
			objIDArray = append(objIDArray, objID)
		}
		filter = bson.M{"_id": bson.M{"$in": objIDArray}}

	} else {

		filter = FullNewClinic{ClinicExtraFields: ClinicExtraFields{Active: true}, NewClinic: newClinic}
	}

	var clinics []Clinic
	if err := c.Store.Find(store.ClinicsCollection, filter, &pagingParams, &clinics); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error finding clinic")
	}

	return ctx.JSON(http.StatusOK, &clinics)
}

// createClinic
// (POST /clinics)
func (c *ClinicServer) PostClinics(ctx echo.Context, params PostClinicsParams) error {

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
func (c *ClinicServer) GetClinicsClinicid(ctx echo.Context, clinicid string, params GetClinicsClinicidParams) error {
	var clinic Clinic
	log.Printf("Get Clinic by id - id: %s", clinicid)
	objID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.M{"_id": objID, "active": true}
	if err := c.Store.FindOne(store.ClinicsCollection, filter, &clinic); err != nil {
		fmt.Println("Find One error ", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error finding clinic")
	}
	clinicsPatientClinician := ClinicsPatientClinician{Clinic: &clinic}

	if params.Clinicians != nil && *params.Clinicians == true {
		pagingParams := store.DefaultPagingParams
		pagingParams.Limit = PATIENT_ALL_PAGING_LIMIT
		clinicsClinicians, err := c.InternalGetClinicsClinicidCliniciansAsUsers(clinicid, pagingParams)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "error accessing clinicians for clinic")
		}
		clinicsPatientClinician.Clinicians = &clinicsClinicians
	}
	if params.Patients != nil && *params.Patients == true {
		pagingParams := store.DefaultPagingParams
		pagingParams.Limit = CLINICIAN_ALL_PAGING_LIMIT
		clinicsPatients, err := c.InternalGetClinicsClinicidPatientsAsUsers(clinicid, pagingParams)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "error accessing patients for clinic")
		}
		clinicsPatientClinician.Patients = &clinicsPatients
	}

	return ctx.JSON(http.StatusOK, &clinicsPatientClinician)
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

// (GET /clinics/access)
func (c * ClinicServer) GetClinicsAccess(ctx echo.Context, params GetClinicsAccessParams) error {
	if params.ClinicId != nil  && params.PatientId != nil {
		// This is an error - can not search by clinician and patient
		return echo.NewHTTPError(http.StatusBadRequest, "Can not filter by both clinicId and patientId ")
	}
	if params.ClinicId == nil  && params.PatientId == nil {
		// This is an error - can not search by clinician and patient
		return echo.NewHTTPError(http.StatusBadRequest, "Must specify clinicId or patientId parameter")
	}
	if params.XTIDEPOOLUSERID == nil || *params.XTIDEPOOLUSERID == "" {
		// Must have userid parameter set
		return echo.NewHTTPError(http.StatusBadRequest, "Must specify X-TIDEPOOL-USERID parameter")
	}

	// Get user id
	pagingParams := store.DefaultPagingParams
	pagingParams.Limit = CLINICS_ALL_PAGING_LIMIT

	// Find all user belongs to

	if params.ClinicId != nil {

		// Just check clinics clinicians collection
		var clinicianClinics []Clinic
		filter := bson.M{"clinicianId": *params.XTIDEPOOLUSERID, "clinicId": *params.ClinicId}
		if err := c.Store.Find(store.ClinicsCliniciansCollection, filter, &pagingParams, &clinicianClinics); err != nil {
			return echo.NewHTTPError(http.StatusForbidden, "Can not access clinic")
		}
		if clinicianClinics == nil {
			return echo.NewHTTPError(http.StatusForbidden, "Can not find any clinics")
		}


		// We found something - access granted
		return ctx.JSON(http.StatusOK, nil)

	} else if params.PatientId != nil {
		// First find user clinics
		var clinicianClinics []ClinicsClinicians
		filter := bson.M{"clinicianId": *params.XTIDEPOOLUSERID}
		if err := c.Store.Find(store.ClinicsCliniciansCollection, filter, &pagingParams, &clinicianClinics); err != nil {
			return echo.NewHTTPError(http.StatusForbidden, "Can not access any clinics")
		}
		if clinicianClinics == nil {
			return echo.NewHTTPError(http.StatusForbidden, "Can not find any clinics")
		}

		// Next find patient clinics
		var patientClinics []ClinicsPatients
		filter = bson.M{"patientId": *params.PatientId}
		if err := c.Store.Find(store.ClinicsPatientsCollection, filter, &pagingParams, &patientClinics); err != nil {
			return echo.NewHTTPError(http.StatusForbidden, "Can not access any patients")
		}
		if patientClinics == nil {
			return echo.NewHTTPError(http.StatusForbidden, "Can not find any patients")
		}

		// Do a set intersection
		for _, clinicianClinic := range(clinicianClinics) {
			for _, patientClinic := range(patientClinics) {
				if *clinicianClinic.ClinicId == *patientClinic.ClinicId {

					// If any matches - return
					return ctx.JSON(http.StatusOK, nil)
				}
			}
		}

	}
	// Find all clinics parient belongs to
	// Set intersection
	return ctx.JSON(http.StatusForbidden, nil)
}

