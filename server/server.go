package server

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"log"
	"net"
	"sync"
)

func ServeAndWait(ctx context.Context, params *Params) error {
	var wg sync.WaitGroup

	grpcLis, err := createListener(params.Cfg.GrpcPort)
	if err != nil {
		return err
	}
	grpcServer := NewGrpcServer()
	grpcServerCtx, _ := context.WithCancel(ctx)

	// Start the grpc server
	wg.Add(1)
	go grpcServer.Run(grpcServerCtx, grpcLis, &wg)

	gatewayLis, err := createListener(params.Cfg.HttpPort)
	if err != nil {
		return err
	}
	gatewayProxy := NewGatewayProxy(params.Cfg)
	gatewayProxyCtx, _ := context.WithCancel(ctx)

	// Connect to the grpc server
	if err := gatewayProxy.Initialize(gatewayProxyCtx, grpcLis.Addr().String()); err == nil {
		// Start the proxy only if it was successfully initialized
		wg.Add(1)
		go gatewayProxy.Run(gatewayProxyCtx, gatewayLis, &wg)
	} else {
		log.Println(fmt.Sprintf("error initializing gateway proxy: %v", err))
		if err := grpcServer.Stop(); err != nil {
			log.Fatalln(fmt.Sprintf("failed to shutdown grpc server: %v", err))
		}
	}

	grpcServer.SetServing()

	// Wait for completion
	wg.Wait()
	return nil
}

func createListener(port uint16) (net.Listener, error) {
	addr := fmt.Sprintf("0.0.0.0:%v", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, errors.Wrap(err, "error creating listener")
	}
	return lis, nil
}
