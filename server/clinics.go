package server

import (
	"context"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/tidepool-org/clinic/api"
	clinic "github.com/tidepool-org/clinic/generated/services"
	models "github.com/tidepool-org/clinic/generated/models"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"fmt"
)

var (
	CLINIC_ADMIN = "CLINIC_ADMIN"
)

type ClinicServer struct {
	Store store.StorageInterface
}

// getClinic
// (GET /clinics)
func (g *GrpcServer) GetClinics(context context.Context, getClinicsRequest *clinic.GetClinicsRequest) (*clinic.GetClinicsResponse, error) {
//func (c *ClinicServer) GetClinics(ctx echo.Context, params GetClinicsParams) error {

	filter :=  bson.M{"active": true}

	// Get Paging params
	pagingParams := store.DefaultPagingParams
	if getClinicsRequest != nil && getClinicsRequest.Limit != 0 {
		pagingParams.Limit = int64(getClinicsRequest.Limit)
	}
	if getClinicsRequest != nil && getClinicsRequest.Offset != 0 {
		pagingParams.Offset = int64(getClinicsRequest.Offset)
	}

	var clinics []models.Clinic
	if err := g.Store.Find(store.ClinicsCollection, filter, &pagingParams, &clinics); err != nil {

		return nil, status.Errorf(codes.Internal, "error finding clinic")
	}

	log.Printf("Clinics found: %v\n", clinics)
	var pClinics []*models.Clinic
	for i := 0; i < len(clinics); i++ {
		pClinics = append(pClinics, &clinics[i])
	}
	log.Printf("Clinics pointers : %v\n", pClinics)

	return &clinic.GetClinicsResponse{
		Data: pClinics,
	}, nil
}

// createClinic
// (POST /clinics)
func (g *GrpcServer) PostClinics(context context.Context, postClinicRequest *clinic.PostClinicsRequest) (*models.ClinicPostId, error) {
	var newClinic api.FullNewClinic

	if postClinicRequest == nil {
		return nil, status.Errorf(codes.Internal, "error no clinic passed in")
	}
	if err := mapModels(postClinicRequest.NewClinic, &newClinic); err != nil {
		return nil, status.Errorf(codes.Internal, "could not read clinic passed in")
	}
	userId := postClinicRequest.XTIDEPOOLUSERID
	newClinic.Active = true

	var clinicsClinicians api.FullClinicsClinicians
	if newID, err := g.Store.InsertOne(store.ClinicsCollection, newClinic); err != nil {
		return nil, status.Errorf(codes.Internal, "Error inserting clinic")
	} else {
		clinicsClinicians.ClinicId = newID
	}

	// We also have to add an initial admin - the user
	//userId := context.Request().Header.Get(TidepoolUserIdHeaderKey)
	clinicsClinicians.Active = true
	clinicsClinicians.ClinicianId = userId
	clinicsClinicians.Permissions = &[]string{CLINIC_ADMIN}
	if _, err := g.Store.InsertOne(store.ClinicsCliniciansCollection, clinicsClinicians); err != nil {
		return nil, status.Errorf(codes.Internal, "Error inserting into clinic/clinician table")
	} else {
		postID := models.ClinicPostId{Id: *clinicsClinicians.ClinicId}
		//postID := PostID{newid: *newID, test: "test"}
		log.Printf("Returning from newer /clinics, %s, %s", postID.Id, *clinicsClinicians.ClinicId)
		return &postID, nil
	}
}

// (DELETE /clinic/{clinicid})
func (g *GrpcServer) DeleteClinicsClinicid(context context.Context, deleteClinicsRequest *clinic.DeleteClinicsClinicidRequest) (*empty.Empty, error) {
	clinicid := deleteClinicsRequest.Clinicid
	objID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.D{{"_id", objID}}
	activeObj := bson.D{
		{"$set", bson.D{{"active", false}}},
	}
	if err := g.Store.UpdateOne(store.ClinicsCollection, filter, activeObj); err != nil {
		return nil, status.Errorf(codes.Internal, "Error deleting clinic from database")
	}
	return new(empty.Empty), nil
}

// getClinic
// (GET /clinic/{clinicid})
func (g *GrpcServer) GetClinicsClinicid(context context.Context, getClinicsRequest *clinic.GetClinicsClinicidRequest) (*clinic.Clinic, error) {
	clinicid := getClinicsRequest.Clinicid
	var dbClinic api.Clinic
	log.Printf("Get Clinic by id - id: %s", clinicid)
	objID, _ := primitive.ObjectIDFromHex(clinicid)
	filter := bson.M{"_id": objID, "active": true}
	if err := g.Store.FindOne(store.ClinicsCollection, filter, &dbClinic); err != nil {
		fmt.Println("Find One error ", err)
		return nil, status.Errorf(codes.Internal, "Error finding clinic")
	}

	var retClinic clinic.Clinic
	if err := mapModels(dbClinic, &retClinic); err != nil {
		return nil, status.Errorf(codes.Internal, "Error retrieving from database")
	}
	return &retClinic, nil
}

func (g *GrpcServer) PatchClinicsClinicid(context context.Context, patchClinicsRequest *clinic.PatchClinicsClinicidRequest) (*empty.Empty, error) {
	var newClinic api.NewClinic
	clinicid := patchClinicsRequest.Clinicid

	if err := mapModels(patchClinicsRequest.NewClinic, &newClinic); err != nil {
		log.Printf("Format failed for patch clinic body")
		return nil, status.Errorf(codes.Internal, "failed retrieving parameters for patch")
	}
	objID, _ := primitive.ObjectIDFromHex(clinicid)
	// XXX what about inactive?
	filter := bson.D{{"_id", objID}}
	patchObj := bson.D{
		{"$set", newClinic },
	}
	if err := g.Store.UpdateOne(store.ClinicsCollection, filter, patchObj); err != nil {
		return nil, status.Errorf(codes.Internal, "could not update database")
	}
	return new(empty.Empty), nil
}
