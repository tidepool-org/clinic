package api

import (
	"fmt"
	"net/http"
)

var (
	StatusMethod = ""
	StatusPath = "/status"
)

func init() {
	InternalRouterMap[RouterKey{StatusMethod, StatusPath}] = InternalStatus
}

func InternalStatus(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "{\"log\": \"Status ok\"}\n")
}
