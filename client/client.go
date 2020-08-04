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
	clinics, err  := c.GetClinics(ctx, &clinic.GetClinicsRequest{})
	fmt.Printf("%v %v\n", clinics, err)
	_, err  = c.PostClinics(ctx, &clinic.PostClinicsRequest{NewClinic: &clinic.NewClinic{Name: "test", Address: "5530 satinleaf way"}})
	fmt.Printf("%v %v\n", clinics, err)
}

