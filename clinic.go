package main

import (
	"context"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/server"
	"log"
	"os"
	"os/signal"
	"syscall"
)

//func main() {
//    api.MainLoop()
//}


func main() {
	cfg := config.New()
	if err := cfg.LoadFromEnv(); err != nil {
		log.Fatalf("could not load service configuration: %v", err)
	}
	params := &server.Params{
		Cfg:   cfg,
	}

	// listen to signals to stop server
	// convert to cancel on context that server listens to
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancelFunc := context.WithCancel(context.Background())
	go func(stop chan os.Signal, cancelFunc context.CancelFunc) {
		<-stop
		log.Print("sigint or sigterm received!!!")
		cancelFunc()
	}(stop, cancelFunc)

	if err := server.ServeAndWait(ctx, params); err != nil {
		log.Fatalln(err.Error())
	}
}
