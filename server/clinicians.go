package server

import (
	"github.com/golang/protobuf/ptypes/empty"
	clinic "github.com/tidepool-org/clinic/generated/services"
	models "github.com/tidepool-org/clinic/generated/models"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/tidepool-org/clinic/api"
	"context"
	"log"
	"fmt"

)

// GetCliniciansFromClinic
// (GET /clinics/{clinicid}/clinicians)
func (g *GrpcServer) GetClinicsClinicidClinicians(context context.Context, getClinicsRequest *clinic.GetClinicsClinicidCliniciansRequest) (*clinic.GetClinicsClinicidCliniciansResponse, error) {
	filter := bson.M{"clinicId": getClinicsRequest.Clinicid, "active": true}

	pagingParams := store.DefaultPagingParams
	if getClinicsRequest != nil && getClinicsRequest.Limit != 0 {
		pagingParams.Limit = int64(getClinicsRequest.Limit)
	}
	if getClinicsRequest != nil && getClinicsRequest.Offset != 0 {
		pagingParams.Offset = int64(getClinicsRequest.Offset)
	}

	var clinicsClinicians []api.ClinicsClinicians
	if err := g.Store.Find(store.ClinicsCliniciansCollection, filter, &pagingParams, &clinicsClinicians); err != nil {
		return nil, status.Errorf(codes.Internal, "error finding clinicians")
	}

	var pClinicsClinicians []*models.ClinicsClinicians
	for i := 0; i < len(clinicsClinicians); i++ {
		var clinicsClinician models.ClinicsClinicians
		if err := mapModels(clinicsClinicians[i], &clinicsClinician); err != nil {
			return nil, status.Errorf(codes.Internal, "error translating clinicians from db")
		}
		pClinicsClinicians = append(pClinicsClinicians, &clinicsClinician)
	}

	return &clinic.GetClinicsClinicidCliniciansResponse{
		Data: pClinicsClinicians,
	}, nil
}

// AddClinicianToClinic
// (POST /clinics/{clinicid}/clinicians)
func (g *GrpcServer) PostClinicsClinicidClinicians(context context.Context, postClinicsRequest *clinic.PostClinicsClinicidCliniciansRequest) (*models.CliniciansPostId, error) {
	var clinicsClinicians api.FullClinicsClinicians

	if err := mapModels(postClinicsRequest, &clinicsClinicians); err != nil {
		log.Printf("Format failed for post clinicsClinicians body")
		return nil, status.Errorf(codes.Internal,"error parsing parameters")
	}
	clinicsClinicians.Active = true
	clinicsClinicians.ClinicId = &postClinicsRequest.Clinicid

	// XXX not sure returning newID makes sense
	if newID, err := g.Store.InsertOne(store.ClinicsCliniciansCollection, clinicsClinicians); err != nil {
		return nil, status.Errorf(codes.Internal,"Error inserting clinician")
	} else {
		postID := models.CliniciansPostId{Id: *newID}
		//postID := PostID{newid: *newID, test: "test"}
		return &postID, nil
	}
}

// DeleteClinicianFromClinic
// (DELETE /clinics/{clinicid}/clinicians/{clinicianid})
func (g *GrpcServer) DeleteClinicsClinicidCliniciansClinicianid(context context.Context, deleteCliniciansRequest *clinic.DeleteClinicsClinicidCliniciansClinicianidRequest) (*empty.Empty, error) {
	// If a clinic administrator - we must ensure that we have one clinic admin object

	// We are a bit clever in determining this - we first try to find 2 admins.  If only one and that one is the
	// one that we are attempting to delete - return an error
	var clinicsClinicians []api.ClinicsClinicians
	filter := bson.M{"clinicId": deleteCliniciansRequest.Clinicid, "active": true, "permissions": CLINIC_ADMIN}
	pagingParams := store.DefaultPagingParams
	pagingParams.Limit = 2
	if err := g.Store.Find(store.ClinicsCliniciansCollection, filter, &pagingParams, &clinicsClinicians); err != nil {
		return nil, status.Errorf(codes.Internal,  "error accessing clinic database")
	}
	if len(clinicsClinicians) == 1 && clinicsClinicians[0].ClinicianId == deleteCliniciansRequest.Clinicianid {
		return nil, status.Errorf(codes.Internal,  "Can not delete last clinic administrator")
	}

	// Passed check - Now delete clinician
	filter = bson.M{"clinicId": deleteCliniciansRequest.Clinicid, "clinicianId": deleteCliniciansRequest.Clinicianid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	if err := g.Store.UpdateOne(store.ClinicsCliniciansCollection, filter, activeObj); err != nil {
		return nil, status.Errorf(codes.Internal, "error deleting clinician from database")
	}
	return nil, nil
}

// GetClinician
// (GET /clinics/{clinicid}/clinicians/{clinicianid})
func (g *GrpcServer) GetClinicsClinicidCliniciansClinicianid(context context.Context, getCliniciansRequest *clinic.GetClinicsClinicidCliniciansClinicianidRequest) (*clinic.ClinicsClinicians, error) {
	var dbClinicsClinicians api.ClinicsClinicians
	log.Printf("Get Clinic by id - id: %s", getCliniciansRequest.Clinicianid)
	filter := bson.M{"clinicId": getCliniciansRequest.Clinicid, "clinicianId": getCliniciansRequest.Clinicianid, "active": true}
	if err := g.Store.FindOne(store.ClinicsCliniciansCollection, filter, &dbClinicsClinicians); err != nil {
		fmt.Println("Find One error ", err)
		return nil, status.Errorf(codes.Internal, "error accessing database")
	}

	var clinicsClinicians clinic.ClinicsClinicians
	if err := mapModels(dbClinicsClinicians, &clinicsClinicians); err != nil {
		return nil, status.Errorf(codes.Internal, "error retrieving from database")
	}
	return &clinicsClinicians, nil
}

// ModifyClinicClinician
// (PATCH /clinics/{clinicid}/clinicians/{clinicianid})
func (g *GrpcServer) PatchClinicsClinicidCliniciansClinicianid(context context.Context, patchCliniciansRequest *clinic.PatchClinicsClinicidCliniciansClinicianidRequest) (*empty.Empty, error) {
	var newClinic api.ClinicianPermissions

	if err := mapModels(patchCliniciansRequest.ClinicianPermissions, &newClinic); err != nil {
		log.Printf("Format failed for patch clinic body")
		return nil, status.Errorf(codes.Internal, "error parsing parameters")
	}
	filter := bson.M{"clinicId": patchCliniciansRequest.Clinicid, "clinicianId": patchCliniciansRequest.Clinicianid}

	patchObj := bson.D{
		{"$set", newClinic },
	}
	if err := g.Store.UpdateOne(store.ClinicsCliniciansCollection, filter, patchObj); err != nil {
		return nil, status.Errorf(codes.Internal,"error updating clinician")
	}
	return nil, nil
}


// Your GET endpoint
// (GET /clinics/clinicians/{clinicianid})
func (g *GrpcServer) GetClinicsCliniciansClinicianid(context context.Context, getCliniciasRequest *clinic.GetClinicsCliniciansClinicianidRequest) (*clinic.GetClinicsCliniciansClinicianidResponse, error) {
	var dbClinicsClinicians []api.ClinicsClinicians
	log.Printf("Get patient by id - id: %s", getCliniciasRequest.Clinicianid)
	filter := bson.M{"clinicianId": getCliniciasRequest.Clinicianid, "active": true}
	pagingParams := store.DefaultPagingParams
	if err := g.Store.Find(store.ClinicsCliniciansCollection, filter, &pagingParams, &dbClinicsClinicians); err != nil {
		fmt.Println("Find error ", err)
		return nil, status.Errorf(codes.Internal, "error finding clinician")
	}

	var pClinicsClinicians []*clinic.ClinicsClinicians
	for i := 0; i < len(dbClinicsClinicians); i++ {
		var clinicsClinicians clinic.ClinicsClinicians
		if err := mapModels(dbClinicsClinicians[i], &clinicsClinicians); err != nil {
			return nil, status.Errorf(codes.Internal, "error translating db info")
		}
		pClinicsClinicians = append(pClinicsClinicians, &clinicsClinicians)
	}
	return &clinic.GetClinicsCliniciansClinicianidResponse{
		Data: pClinicsClinicians,
	}, nil
}

// (DELETE /clinics/clinicians/{clinicianid})
func (g *GrpcServer) DeleteClinicsCliniciansClinicianid(context context.Context, deleteCliniciansRequest *clinic.DeleteClinicsCliniciansClinicianidRequest) (*empty.Empty, error) {
	filter := bson.M{ "clinicianId": deleteCliniciansRequest.Clinicianid}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	if err := g.Store.Update(store.ClinicsCliniciansCollection, filter, activeObj); err != nil {
		return nil, status.Errorf(codes.Internal, "error deleting clinician from clinic")
	}
	return nil, nil

}
