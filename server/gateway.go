package server

import (
	"context"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/generated/services"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
	"sync"
)

type GatewayProxy struct {
	httpServer *http.Server
	mux        *runtime.ServeMux
	cfg        *config.Config
}

func NewGatewayProxy(cfg *config.Config) *GatewayProxy {
	return &GatewayProxy{
		cfg: cfg,
	}
}

func (g *GatewayProxy) Initialize(ctx context.Context, endpoint string) error {
	mux := runtime.NewServeMux()
	err := clinic.RegisterDefaultServiceHandlerFromEndpoint(ctx, mux, endpoint, []grpc.DialOption{grpc.WithInsecure()})
	if err != nil {
		return err
	}
	g.mux = mux
	swagger, err := api.GetSwagger()
	authClient := api.AuthClient{}
	filterOptions := openapi3filter.Options{AuthenticationFunc: authClient.AuthenticationFunc}
	options := Options{Options: filterOptions}
	g.httpServer = &http.Server{
		//Handler: http.HandlerFunc(OapiRequestValidator2(mux.ServeHTTP)),
		Handler: OapiRequestValidator2(swagger, &options, http.HandlerFunc(mux.ServeHTTP)),
	}
	return nil
}

func (g *GatewayProxy) Run(ctx context.Context, lis net.Listener, wg *sync.WaitGroup) {
	go func() {
		<-ctx.Done()
		if err := g.stop(wg); err != nil {
			log.Println(fmt.Sprintf("error while shutting down the gateway proxy: %v", err))
		} else {
			log.Println("gateway proxy was successfully shutdown")
		}
	}()

	log.Println(fmt.Sprintf("serving http requests on %v", lis.Addr()))
	if err := g.httpServer.Serve(lis); err != nil && err != http.ErrServerClosed {
		log.Println(fmt.Printf("error serving http: %v", err))
	}
}

func (g *GatewayProxy) stop(wg *sync.WaitGroup) error {
	defer wg.Done()
	return g.httpServer.Shutdown(context.Background())
}
