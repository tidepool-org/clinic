package test

import (
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
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
	TestUserId               = "1234567890"
	TestServiceAccountUserId = "9999999999"
	TestServiceAccountToken  = "service-account"
	TestXealthUserId         = "1234567891"
	TestUserToken            = "user"
	TestServerId             = "server"
	TestServerToken          = "server"
	TestRestrictedToken      = "1234567890abcdef1234567890abcdef"
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
		} else if r.Method == http.MethodGet && r.RequestURI == fmt.Sprintf("/token/%s", TestServiceAccountToken) {
			resp, _ = json.Marshal(shoreline.TokenData{
				UserID:   TestServiceAccountUserId,
				IsServer: false,
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
	tokens := make(map[string]auth.RestrictedToken)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp []byte
		if r.Method == http.MethodPost && createRestrictedTokenUrlRegexp.MatchString(r.RequestURI) {
			matches := createRestrictedTokenUrlRegexp.FindStringSubmatch(r.RequestURI)
			tokens[TestRestrictedToken] = auth.RestrictedToken{
				ID:             TestRestrictedToken,
				UserID:         matches[1],
				ExpirationTime: time.Now().Add(time.Hour),
				CreatedTime:    time.Now(),
			}
			resp, _ = json.Marshal(tokens[TestRestrictedToken])
		} else if r.Method == http.MethodGet && r.RequestURI == "/v1/restricted_tokens/1234567890abcdef1234567890abcdef" {
			resp, _ = json.Marshal(tokens[TestRestrictedToken])
		} else {
			w.WriteHeader(http.StatusNotImplemented)
		}

		w.Write(resp)
	}))
}

func KeycloakStub() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp []byte
		if r.Method == http.MethodPost && r.RequestURI == "/realms/integration-test/protocol/openid-connect/token" {
			if err := r.ParseForm(); err != nil {
				w.WriteHeader(http.StatusNotImplemented)
			} else {
				if r.Form.Get("grant_type") == "client_credentials" {
					iat := &jwt.NumericDate{Time: time.Now()}
					exp := &jwt.NumericDate{Time: time.Now().Add(120 * time.Second)}
					claims := jwt.RegisteredClaims{
						Issuer:    "https://integeation-test.com",
						Subject:   TestServiceAccountUserId,
						Audience:  []string{"integration-test"},
						ExpiresAt: exp,
						NotBefore: iat,
						IssuedAt:  iat,
						ID:        uuid.New().String(),
					}
					j := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
					token, _ := j.SignedString([]byte("test"))

					response := struct {
						IdToken          string `json:"id_token"`
						AccessToken      string `json:"access_token"`
						RefreshToken     string `json:"refresh_token"`
						ExpiresIn        int    `json:"expires_in"`
						RefreshExpiresIn int    `json:"refresh_expires_in"`
						Scope            string `json:"scope"`
					}{
						IdToken:          token,
						AccessToken:      token,
						RefreshToken:     token,
						ExpiresIn:        120,
						RefreshExpiresIn: 120,
						Scope:            "openid",
					}
					resp, _ = json.Marshal(response)
					w.Header().Set("content-type", "application/json")
				}
			}
		} else {
			w.WriteHeader(http.StatusNotImplemented)
		}

		w.Write(resp)
	}))
}
