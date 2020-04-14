package api

import (
	"fmt"
	"net/http"
)

var (
	QueryClinicMethod = "GET"
	QueryClinicPath = "/clinic"
)

func init() {
	SpecRouterMap[RouterKey{QueryClinicMethod, QueryClinicPath}] = Create_clinic
}

func Query_clinic(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "{\"log\": \"Query Clinic called\"}\n")
}
