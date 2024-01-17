package test

import (
	"encoding/json"
	"fmt"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/platform/auth"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"time"
)

const (
	TestUserId       = "1234567890"
	TestXealthUserId = "1234567891"
	TestUserToken    = "user"
	TestServerId     = "server"
	TestServerToken  = "server"
)

var (
	xealthUser = shoreline.UserData{
		UserID:   TestXealthUserId,
		Username: "xealth@tidepool.org",
		Emails: []string{
			"xealth@tidepool.org",
		},
		PasswordExists: true,
		Roles:          []string{"patient"},
		EmailVerified:  true,
	}

	clinicianUser = shoreline.UserData{
		UserID:   TestUserId,
		Username: "test@tidepool.org",
		Emails: []string{
			"test@tidepool.org",
		},
		PasswordExists: true,
		Roles:          []string{"clinician"},
		EmailVerified:  true,
	}

	createClinicUserUrlRegexp      = regexp.MustCompile("/v1/clinics/.+/users")
	createRestrictedTokenUrlRegexp = regexp.MustCompile("/v1/users/(.+)/restricted_tokens")
)

func ShorelineStub() *httptest.Server {
	xealthPatientCreated := false
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp []byte
		if r.Method == http.MethodGet && r.RequestURI == fmt.Sprintf("/token/%s", TestUserToken) {
			resp, _ = json.Marshal(shoreline.TokenData{
				UserID:   TestUserId,
				IsServer: false,
			})
		} else if r.Method == http.MethodGet && r.RequestURI == fmt.Sprintf("/token/%s", TestServerToken) {
			resp, _ = json.Marshal(shoreline.TokenData{
				UserID:   TestServerId,
				IsServer: true,
			})
		} else if r.Method == http.MethodGet && r.RequestURI == fmt.Sprintf("/user/%s", TestUserId) {
			resp, _ = json.Marshal(clinicianUser)
		} else if r.Method == http.MethodGet && r.RequestURI == fmt.Sprintf("/user/%s", "xealth@tidepool.org") {
			if !xealthPatientCreated {
				w.WriteHeader(http.StatusNotFound)
			} else {
				resp, _ = json.Marshal(xealthUser)
			}
		} else if r.Method == http.MethodPost && createClinicUserUrlRegexp.MatchString(r.RequestURI) {
			xealthPatientCreated = true
			resp, _ = json.Marshal(xealthUser)
			w.WriteHeader(http.StatusCreated)
		} else if r.Method == http.MethodPost && strings.HasSuffix(r.RequestURI, "/serverlogin") {
			w.Header().Set("x-tidepool-session-token", "server")
		} else {
			w.WriteHeader(http.StatusNotFound)
		}

		w.Write(resp)
	}))
}

func SeagullStub() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp []byte
		if r.Method == http.MethodGet && r.RequestURI == fmt.Sprintf("/%s/profile", TestUserId) {
			fullName := "Clinician 1"
			resp, _ = json.Marshal(patients.Profile{
				FullName: &fullName,
			})
		} else {
			w.WriteHeader(http.StatusNotImplemented)
		}

		w.Write(resp)
	}))
}

func AuthStub() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp []byte
		if r.Method == http.MethodPost && createRestrictedTokenUrlRegexp.MatchString(r.RequestURI) {
			matches := createRestrictedTokenUrlRegexp.FindStringSubmatch(r.RequestURI)
			resp, _ = json.Marshal(auth.RestrictedToken{
				ID:             "1234567890abcdef1234567890abcdef",
				UserID:         matches[1],
				ExpirationTime: time.Now().Add(time.Hour),
				CreatedTime:    time.Now(),
			})
		} else {
			w.WriteHeader(http.StatusNotImplemented)
		}

		w.Write(resp)
	}))
}
