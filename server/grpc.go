
package server

import (
	"encoding/json"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/generated/services"
	"github.com/tidepool-org/clinic/store"

	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpcHealth "google.golang.org/grpc/health/grpc_health_v1"
	grpcReflection "google.golang.org/grpc/reflection"
	"log"
	"net"
	"sync"
)

const serviceName = "clinic"

type Params struct {
	Cfg   *config.Config
}

type GrpcServer struct {
	grpcServer   *grpc.Server
	healthServer *health.Server

	Store store.StorageInterface
}

//var _ api.DevicesServer = &GrpcServer{}

func NewGrpcServer() *GrpcServer {
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(UnaryServerInterceptor()))
	healthServer := health.NewServer()

	// Connection string
	mongoHost, err := store.GetConnectionString()
	if err != nil {
		log.Fatal("Cound not connect to database: ", err)
	}

	// Create Store
	log.Println("Getting Mongog Store")
	dbstore := store.NewMongoStoreClient(mongoHost)

	srvr := &GrpcServer{
		grpcServer:   grpcServer,
		healthServer: healthServer,
		Store: dbstore,
	}

	clinic.RegisterDefaultServiceServer(grpcServer, srvr)
	//api.RegisterDevicesServer(grpcServer, srvr)
	grpcHealth.RegisterHealthServer(grpcServer, healthServer)
	grpcReflection.Register(grpcServer)


	return srvr
}

func (s *GrpcServer) Run(ctx context.Context, lis net.Listener, wg *sync.WaitGroup) {
	defer wg.Done()

	go func() {
		<-ctx.Done()
		if err := s.Stop(); err != nil {
			log.Println(fmt.Sprintf("error while shutting down the grpc server: %v", err))
		}
	}()

	log.Println(fmt.Sprintf("serving grpc requests on %v", lis.Addr().String()))
	// blocks until the grpc server exits
	if err := s.grpcServer.Serve(lis); err != nil {
		log.Println(fmt.Sprintf("failed to start grpc server: %v", err))
		return
	}

	log.Println("grpc server was successfully shutdown")
}

func (s *GrpcServer) Stop() error {
	s.SetNotServing()
	s.grpcServer.GracefulStop()
	return nil
}

func (s *GrpcServer) SetServing() {
	s.healthServer.SetServingStatus(serviceName, grpcHealth.HealthCheckResponse_SERVING)
}

func (s *GrpcServer) SetNotServing() {
	s.healthServer.SetServingStatus(serviceName, grpcHealth.HealthCheckResponse_NOT_SERVING)
}

func mapModels(from interface{}, to interface{}) error {
	s, err := json.Marshal(&from)
	fmt.Printf("Marhalled: %s\n", s)
	if err != nil {
		return err
	}
	json.Unmarshal(s, to)
	return nil
}

