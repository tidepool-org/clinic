package main

import (
	"github.com/tidepool-org/clinic/api"
	_ "github.com/tidepool-org/clinic/client"
)

func main() {
    api.MainLoop()
}
