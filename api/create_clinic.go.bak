package api

import (
	"fmt"
	"net/http"
)

var (
	CreateClinicMethod = "POST"
	CreateClinicPath = "/clinic"
)

func init() {
	SpecRouterMap[RouterKey{CreateClinicMethod, CreateClinicPath}] = Create_clinic
}

func Create_clinic(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "{\"log\": \"Create Clinic called\"}\n")
}
