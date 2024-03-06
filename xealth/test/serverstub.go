package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
}

func (x *XealthServer) AddOrder(deployment, orderId string, orderBody []byte) {
	if x.orders == nil {
		x.orders = make(map[string][]byte)
	}
	orderPath := fmt.Sprintf("%s/%s", deployment, orderId)
	x.orders[orderPath] = orderBody
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
