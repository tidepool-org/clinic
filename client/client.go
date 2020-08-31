package main

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/generated/services"
	"google.golang.org/grpc"
	"log"
)

const (
	address = "localhost:50051"
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := clinic.NewDefaultServiceClient(conn)

	ctx := context.Background()

	clinicId := createClinics(c, ctx)

	patchClinics(c, ctx, clinicId)
}

func createClinics(c clinic.DefaultServiceClient, ctx context.Context) string  {
	ret, err  := c.PostClinics(ctx, &clinic.PostClinicsRequest{NewClinic: &clinic.NewClinic{Name: "test", Address: "5530 satinleaf way"}, XTIDEPOOLUSERID: "User1"})
	fmt.Printf("Post: %v E:%v\n", ret, err)
	clinics, err  := c.GetClinics(ctx, &clinic.GetClinicsRequest{})
	fmt.Printf("Get: %v E:%v\n", clinics, err)
	return ret.ClinicId
}

func patchClinics(c clinic.DefaultServiceClient, ctx context.Context, clinicId string)  {
	_, err := c.PatchClinicsClinicid(ctx, &clinic.PatchClinicsClinicidRequest{Clinicid: clinicId, NewClinic: &clinic.NewClinic{Name: "Modified Clinic"}})
	fmt.Printf("Patch: E:%v\n", err)
}

