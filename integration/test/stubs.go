package test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/tidepool-org/platform/auth"
)

const (
	TestUserId               = "1234567890"
	TestLegacyClinicUserId   = "2345678901"
	TestServiceAccountUserId = "9999999999"
	TestServiceAccountToken  = "service-account"
	TestXealthUserId         = "1234567891"
	TestXealthGuardianUserId = "1234567893"
	TestRedoxUserId          = "1234567892"
	TestUserToken            = "user"
	TestLegacyClinicToken    = "clinic"
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

	xealthGuardianUser = shoreline.UserData{
		UserID:   TestXealthGuardianUserId,
		Username: "xealth+guardian@tidepool.org",
		Emails: []string{
			"xealth+guardian@tidepool.org",
		},
		PasswordExists: true,
		Roles:          []string{"patient"},
		EmailVerified:  true,
	}

	redoxUser = shoreline.UserData{
		UserID:   TestRedoxUserId,
		Username: "redox@tidepool.org",
		Emails: []string{
			"redox@tidepool.org",
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

	clinicUser = shoreline.UserData{
		UserID:   TestLegacyClinicUserId,
		Username: "clinic@tidepool.org",
		Emails: []string{
			"clinic@tidepool.org",
		},
		PasswordExists: true,
		Roles:          []string{"clinic"},
		EmailVerified:  true,
	}

	// Users which are created lazily by the custodial user creation endpoint,
	// preserving the behavior the xealth and redox specs depend on: a lookup
	// by username returns 404 until the user has been created. The guardian
	// user is intentionally never registered for lookups.
	predefinedUsers = map[string]struct {
		user     shoreline.UserData
		register bool
	}{
		"xealth@tidepool.org":          {user: xealthUser, register: true},
		"redox@tidepool.org":           {user: redoxUser, register: true},
		"xealth+guardian@tidepool.org": {user: xealthGuardianUser, register: false},
	}

	createClinicUserUrlRegexp      = regexp.MustCompile("/v1/clinics/.+/users")
	createRestrictedTokenUrlRegexp = regexp.MustCompile("/v1/users/(.+)/restricted_tokens")
)

// StubUsers is a registry of Tidepool users backing ShorelineStub and
// SeagullStub. It is pre-seeded with the fixed users and tokens the existing
// specs rely on, and allows specs to register additional users so tests can
// authenticate as arbitrary clinicians and patients or create custodial
// accounts with arbitrary emails.
type StubUsers struct {
	mu         sync.Mutex
	byId       map[string]shoreline.UserData
	byUsername map[string]shoreline.UserData
	tokens     map[string]shoreline.TokenData
	tokenById  map[string]string
	profiles   map[string]patients.Profile
	nextId     int64
}

func NewStubUsers() *StubUsers {
	u := &StubUsers{}
	u.Reset()
	return u
}

// Reset restores the registry to its initial seeded state. The suite database
// is shared across specs, so this is not invoked automatically; specs must
// keep their data unique instead.
func (u *StubUsers) Reset() {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.byId = map[string]shoreline.UserData{}
	u.byUsername = map[string]shoreline.UserData{}
	u.tokens = map[string]shoreline.TokenData{}
	u.tokenById = map[string]string{}
	u.profiles = map[string]patients.Profile{}
	u.nextId = 5000000000

	u.registerLocked(clinicianUser, TestUserToken, false)
	u.registerLocked(clinicUser, TestLegacyClinicToken, true)
	u.tokens[TestServerToken] = shoreline.TokenData{UserID: TestServerId, IsServer: true}
	u.tokens[TestServiceAccountToken] = shoreline.TokenData{UserID: TestServiceAccountUserId, IsServer: false}

	clinicianName := "Clinician 1"
	clinicName := "Clinic 1"
	u.profiles[TestUserId] = patients.Profile{FullName: &clinicianName}
	u.profiles[TestLegacyClinicUserId] = patients.Profile{FullName: &clinicName}
}

func (u *StubUsers) registerLocked(user shoreline.UserData, token string, isServer bool) {
	u.byId[user.UserID] = user
	if user.Username != "" {
		u.byUsername[user.Username] = user
	}
	u.tokens[token] = shoreline.TokenData{UserID: user.UserID, IsServer: isServer}
	u.tokenById[user.UserID] = token
}

// AddUser registers an existing Tidepool user and returns a session token
// which authenticates as that user.
func (u *StubUsers) AddUser(user shoreline.UserData) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	token := "token-" + user.UserID
	u.registerLocked(user, token, false)
	return token
}

// NextUserId returns a new unique 10-digit user id.
func (u *StubUsers) NextUserId() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.nextId++
	return strconv.FormatInt(u.nextId, 10)
}

// CreateUser backs the custodial account creation endpoint. Usernames of
// already registered users return the existing user, mirroring the previous
// stub behavior of always responding with a fixed user.
func (u *StubUsers) CreateUser(username string) shoreline.UserData {
	u.mu.Lock()
	defer u.mu.Unlock()

	if username != "" {
		if existing, ok := u.byUsername[username]; ok {
			return existing
		}
		if predefined, ok := predefinedUsers[username]; ok {
			if predefined.register {
				u.registerLocked(predefined.user, "token-"+predefined.user.UserID, false)
			}
			return predefined.user
		}
	}

	u.nextId++
	user := shoreline.UserData{
		UserID:   strconv.FormatInt(u.nextId, 10),
		Username: username,
	}
	if username != "" {
		user.Emails = []string{username}
	}
	u.registerLocked(user, "token-"+user.UserID, false)
	return user
}

func (u *StubUsers) User(idOrUsername string) (shoreline.UserData, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if user, ok := u.byId[idOrUsername]; ok {
		return user, true
	}
	user, ok := u.byUsername[idOrUsername]
	return user, ok
}

func (u *StubUsers) Token(token string) (shoreline.TokenData, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	data, ok := u.tokens[token]
	return data, ok
}

// TokenFor returns the session token of a registered user, or an empty string
// when the user is unknown.
func (u *StubUsers) TokenFor(userId string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.tokenById[userId]
}

// SetProfile overrides the seagull profile of a user.
func (u *StubUsers) SetProfile(userId string, profile patients.Profile) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.profiles[userId] = profile
}

// Profile returns the explicitly set profile of a user, or a generic profile
// for any registered user.
func (u *StubUsers) Profile(userId string) (patients.Profile, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if profile, ok := u.profiles[userId]; ok {
		return profile, true
	}
	if _, ok := u.byId[userId]; ok {
		fullName := "User " + userId
		return patients.Profile{FullName: &fullName}, true
	}
	return patients.Profile{}, false
}

func writeJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	resp, _ := json.Marshal(v)
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(resp)
}

func ShorelineStub(users *StubUsers) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/token/"):
			token := strings.TrimPrefix(r.URL.Path, "/token/")
			if data, ok := users.Token(token); ok {
				writeJSON(w, http.StatusOK, data)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/user/"):
			idOrUsername := strings.TrimPrefix(r.URL.Path, "/user/")
			if user, ok := users.User(idOrUsername); ok {
				writeJSON(w, http.StatusOK, user)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case r.Method == http.MethodPost && createClinicUserUrlRegexp.MatchString(r.URL.Path):
			payload := struct {
				Username string `json:"username"`
			}{}
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &payload)
			writeJSON(w, http.StatusCreated, users.CreateUser(payload.Username))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/serverlogin"):
			w.Header().Set("x-tidepool-session-token", TestServerToken)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func SeagullStub(users *StubUsers) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/profile") {
			userId := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), "/profile")
			if profile, ok := users.Profile(userId); ok {
				writeJSON(w, http.StatusOK, profile)
				return
			}
		}
		w.WriteHeader(http.StatusNotImplemented)
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
