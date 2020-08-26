package server

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/tidepool-org/clinic/api"
	models "github.com/tidepool-org/clinic/generated/models"
	clinic "github.com/tidepool-org/clinic/generated/services"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
)


type ClinicsPatientsExtraFields struct {
	Active bool `json:"active" bson:"active"`
}


type FullClinicsPatients struct {
	api.ClinicsPatients `bson:",inline"`
	ClinicsPatientsExtraFields `bson:",inline"`
}

// GetPatientsForClinic
// (GET /clinics/{clinicid}/patients)
func (g *GrpcServer) GetClinicsClinicidPatients(context context.Context, getPatientsRequest *clinic.GetClinicsClinicidPatientsRequest) (*clinic.GetClinicsClinicidPatientsResponse, error) {
	filter := bson.M{"clinicId": getPatientsRequest.Clinicid, "active": true}

	pagingParams := store.DefaultPagingParams
	if getPatientsRequest != nil && getPatientsRequest.Limit != 0 {
		pagingParams.Limit = int64(getPatientsRequest.Limit)
	}
	if getPatientsRequest != nil && getPatientsRequest.Offset != 0 {
		pagingParams.Offset = int64(getPatientsRequest.Offset)
	}

	var clinicsPatients []api.ClinicsPatients
	if err := g.Store.Find(store.ClinicsPatientsCollection, filter, &pagingParams, &clinicsPatients); err != nil {
		return nil, status.Errorf(codes.Internal, "error accessing database")
	}

	var pClinicsPatients []*models.ClinicsPatients
	for i := 0; i < len(clinicsPatients); i++ {
		var clinicsPatient models.ClinicsPatients
		if err := mapModels(clinicsPatients[i], &clinicsPatients); err != nil {
			return nil, status.Errorf(codes.Internal, "error translating clinicians from db")
		}
		pClinicsPatients = append(pClinicsPatients, &clinicsPatient)
	}


	return &clinic.GetClinicsClinicidPatientsResponse{
		Data: pClinicsPatients,
	}, nil
}

// AddPatientToClinic
// (POST /clinics/{clinicid}/patients)
func (g *GrpcServer) PostClinicsClinicidPatients(context context.Context, postPatientsRequest *clinic.PostClinicsClinicidPatientsRequest) (*clinic.PatientPostId, error) {
	var clinicsPatients FullClinicsPatients

	if err := mapModels(postPatientsRequest, &clinicsPatients); err != nil {
		log.Printf("Format failed for post clinicsClinicians body")
		return nil, status.Errorf(codes.Internal,"error parsing parameters")
	}
	clinicsPatients.Active = true
	clinicsPatients.ClinicId = &postPatientsRequest.Clinicid

	if newID, err := g.Store.InsertOne(store.ClinicsPatientsCollection, clinicsPatients); err != nil {
		return nil, status.Errorf(codes.Internal, "error inserting patient ")
	} else {
		return &clinic.PatientPostId{Id: *newID}, nil
	}
}

// DeletePatientFromClinic
// (DELETE /clinics/{clinicid}/patients/{patientid})
func (g *GrpcServer) DeleteClinicClinicidPatientsPatientid(context context.Context, deletePatientRequest *clinic.DeleteClinicClinicidPatientsPatientidRequest) (*empty.Empty, error) {
	filter := bson.M{"clinicId": deletePatientRequest.Clinicid, "patientId": deletePatientRequest.Patientid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	if err := g.Store.UpdateOne(store.ClinicsPatientsCollection, filter, activeObj); err != nil {
		return nil, status.Errorf(codes.Internal, "error deleting patient from clinic")
	}
	return new(empty.Empty), nil
}


// GetPatientFromClinic
// (GET /clinics/{clinicid}/patients/{patientid})
func (g *GrpcServer) GetClinicsClinicidPatientsPatientid(context context.Context, getPatientsRequest *clinic.GetClinicsClinicidPatientsPatientidRequest) (*clinic.ClinicsPatients, error) {
	var dbClinicsPatients api.ClinicsPatients
	log.Printf("Get Clinic by id - id: %s", getPatientsRequest.Clinicid)
	filter := bson.M{"clinicId": getPatientsRequest.Clinicid, "patientId": getPatientsRequest.Patientid, "active": true}
	if err := g.Store.FindOne(store.ClinicsPatientsCollection, filter, &dbClinicsPatients); err != nil {
		fmt.Println("Find One error ", err)
		return nil, status.Errorf(codes.Internal,"error finding patient")
	}

	var clinicsPatients clinic.ClinicsPatients
	if err := mapModels(dbClinicsPatients, &clinicsPatients); err != nil {
		return nil, status.Errorf(codes.Internal, "error retrieving from database")
	}

	return &clinicsPatients, nil
}

// ModifyClinicPatient
// (PATCH /clinics/{clinicid}/patients/{patientid})
func (g *GrpcServer) PatchClinicsClinicidPatientsPatientid(context context.Context, patchPatientsRequest *clinic.PatchClinicsClinicidPatientsPatientidRequest) (*empty.Empty, error) {
	var newPatient api.PatientPermissions
	if err := mapModels(patchPatientsRequest.PatientPermissions, &newPatient); err != nil {
		log.Printf("Format failed for patch clinic body")
		return nil, status.Errorf(codes.Internal, "error parsing parameters")
	}
	filter := bson.M{"clinicId": patchPatientsRequest.Clinicid, "patientId": patchPatientsRequest.Patientid}

	patchObj := bson.D{
		{"$set", newPatient },
	}
	if err := g.Store.UpdateOne(store.ClinicsPatientsCollection, filter, patchObj); err != nil {
		return  nil, status.Errorf(codes.Internal, "error updating patient")
	}
	return new(empty.Empty), nil
}

// Your GET endpoint
// (GET /clinics/patients/{patientid})
func (g *GrpcServer) GetClinicsPatientsPatientid(context context.Context, getPatientsReequest *clinic.GetClinicsPatientsPatientidRequest) (*clinic.GetClinicsPatientsPatientidResponse, error) {

	var dbClinicsPatients []api.ClinicsPatients
	log.Printf("Get patient by id - id: %s", getPatientsReequest.Patientid)
	filter := bson.M{"patientId": getPatientsReequest.Patientid, "active": true}
	pagingParams := store.DefaultPagingParams
	if err := g.Store.Find(store.ClinicsPatientsCollection, filter, &pagingParams, &dbClinicsPatients); err != nil {
		fmt.Println("Find error ", err)
		return nil, status.Errorf(codes.Internal, "error finding patient")
	}

	var pClinicsPatients []*clinic.ClinicsPatients
	for i := 0; i < len(dbClinicsPatients); i++ {
		var clinicsPatients clinic.ClinicsPatients
		if err := mapModels(dbClinicsPatients[i], &clinicsPatients); err != nil {
			return nil, status.Errorf(codes.Internal, "error translating db info")
		}
		pClinicsPatients = append(pClinicsPatients, &clinicsPatients)
	}
	return &clinic.GetClinicsPatientsPatientidResponse{
		Data: pClinicsPatients,
	}, nil

}

// (DELETE /clinics/patients/{patientid})
func (g *GrpcServer) DeleteClinicsPatientsPatientid(context context.Context, deletePatientsRequest *clinic.DeleteClinicsPatientsPatientidRequest) (*empty.Empty, error) {
	filter := bson.M{ "patientId": deletePatientsRequest.Patientid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	if err := g.Store.Update(store.ClinicsPatientsCollection, filter, activeObj); err != nil {
		return nil, status.Errorf(codes.Internal, "error deleting patient from clinic")
	}
	return new(empty.Empty), nil
}