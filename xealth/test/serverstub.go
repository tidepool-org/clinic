package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

const (
	XealthBearerToken  = "xealth-token"
	XealthOauth2Token  = "oauth2-token"
	XealthClientId     = "client-id"
	XealthClientSecret = "client-secret"
	TokenEndpoint      = "/oauth2/token"
)

type XealthServer struct {
	*httptest.Server
	orders map[string][]byte

	mu                sync.Mutex
	observations      [][]byte
	observationStatus int
}

func (x *XealthServer) AddOrder(deployment, orderId string, orderBody []byte) {
	if x.orders == nil {
		x.orders = make(map[string][]byte)
	}
	orderPath := fmt.Sprintf("%s/%s", deployment, orderId)
	x.orders[orderPath] = orderBody
}

// SetObservationStatus overrides the HTTP status returned for FHIR Observation
// POSTs (default 201). Use it to simulate Xealth rejecting an observation.
func (x *XealthServer) SetObservationStatus(status int) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.observationStatus = status
}

// Observations returns the bodies of all captured FHIR Observation POSTs.
func (x *XealthServer) Observations() [][]byte {
	x.mu.Lock()
	defer x.mu.Unlock()
	return append([][]byte(nil), x.observations...)
}

// ResetObservations clears the captured FHIR Observation POSTs.
func (x *XealthServer) ResetObservations() {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.observations = nil
}

func (x *XealthServer) recordObservation(body []byte) int {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.observations = append(x.observations, body)
	if x.observationStatus != 0 {
		return x.observationStatus
	}
	return http.StatusCreated
}

func ServerStub() *XealthServer {
	xealth := &XealthServer{}
	xealth.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && xealth.orders != nil && strings.HasPrefix(r.RequestURI, "/partner/read/order/") {
			orderPath, _ := strings.CutPrefix(r.RequestURI, "/partner/read/order/")
			if orderBody, ok := xealth.orders[orderPath]; ok {
				w.Header().Add("content-type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(orderBody)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		} else if r.Method == http.MethodPost && strings.HasPrefix(r.RequestURI, "/partner/fhir/R4/") && strings.HasSuffix(r.RequestURI, "/Observation") {
			body, _ := io.ReadAll(r.Body)
			status := xealth.recordObservation(body)
			w.Header().Add("content-type", "application/json")
			w.WriteHeader(status)
			w.Write(body)
		} else if r.Method == http.MethodPost && r.RequestURI == TokenEndpoint {
			token := map[string]interface{}{
				"access_token": XealthOauth2Token,
				"expires_in":   3600,
			}
			body, err := json.Marshal(token)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Add("content-type", "application/json")
			w.Write(body)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return xealth
}
