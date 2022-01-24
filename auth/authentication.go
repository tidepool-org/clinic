package auth

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"github.com/hashicorp/golang-lru/simplelru"
	"net/http"
	"sync"
	"time"
)

var (
	ErrUnauthenticated            = fmt.Errorf("session token is invalid")
	AuthContextKey                = AuthKey("auth")
	TidepoolSessionTokenHeaderKey = "x-tidepool-session-token"
	DefaultCacheSize              = 10000           // Cache up to 10000 tokens
	DefaultCacheEntryExpiration   = 5 * time.Minute // Cache tokens for 5 minutes
)

type AuthKey string

type Auth struct {
	SubjectId    string `json:"subjectId"`
	ServerAccess bool   `json:"serverAccess"`
}

func IsServerAuth(a *Auth) bool {
	return a != nil && a.ServerAccess
}

type Authenticator interface {
	ValidateAndSetAuthData(token string, ec echo.Context) (bool, error)
}

type ShorelineAuthenticator struct {
	shoreline shoreline.Client
}

var _ Authenticator = &ShorelineAuthenticator{}

type AuthMiddlewareOpts struct {
	Skipper middleware.Skipper
}

func NewAuthMiddleware(authenticator Authenticator, opts AuthMiddlewareOpts) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Allow skipping authentication for certain routes (e.g. readiness probe)
			if opts.Skipper != nil {
				if opts.Skipper(c) {
					return next(c)
				}
			}

			token := c.Request().Header.Get(TidepoolSessionTokenHeaderKey)
			if token == "" {
				return echo.NewHTTPError(http.StatusBadRequest, "session token is missing")
			}

			valid, err := authenticator.ValidateAndSetAuthData(token, c)
			if err != nil {
				return &echo.HTTPError{
					Code:     http.StatusUnauthorized,
					Message:  "session token is invalid",
					Internal: err,
				}
			} else if valid {
				return next(c)
			}
			return echo.ErrUnauthorized
		}
	}
}

// NewAuthenticator returns a shoreline authenticator that caches server tokens
func NewAuthenticator(shoreline shoreline.Client) (Authenticator, error) {
	delegate := NewShorelineAuthenticator(shoreline)
	return NewCachingAuthenticator(
		DefaultCacheSize,
		DefaultCacheEntryExpiration,
		delegate,
		IsServerAuth,
	)
}

func NewShorelineAuthenticator(shoreline shoreline.Client) Authenticator {
	return &ShorelineAuthenticator{shoreline: shoreline}
}

func (s *ShorelineAuthenticator) ValidateAndSetAuthData(token string, ec echo.Context) (bool, error) {
	data := s.shoreline.CheckToken(token)
	if data != nil && data.UserID != "" {
		SetAuthData(ec, &Auth{
			SubjectId:    data.UserID,
			ServerAccess: data.IsServer,
		})
		return true, nil
	}

	return false, ErrUnauthenticated
}

func GetAuthData(ctx context.Context) *Auth {
	if auth, ok := ctx.Value(AuthContextKey).(*Auth); ok {
		return auth
	}

	return nil
}

func SetAuthData(ec echo.Context, auth *Auth) {
	ctx := context.WithValue(ec.Request().Context(), AuthContextKey, auth)
	ec.SetRequest(ec.Request().WithContext(ctx))
}

type CacheEntry struct {
	token  string
	auth   *Auth
	expiry time.Time
}

func (c CacheEntry) IsExpired() bool {
	return time.Now().After(c.expiry)
}

type CachingAuthenticator struct {
	delegate    Authenticator
	expiration  time.Duration
	lru         *simplelru.LRU
	mu          *sync.Mutex
	shouldCache func(*Auth) bool
}

var _ Authenticator = &CachingAuthenticator{}

func NewCachingAuthenticator(size int, expiration time.Duration, delegate Authenticator, shouldCache func(*Auth) bool) (Authenticator, error) {
	var onEvict simplelru.EvictCallback
	lru, err := simplelru.NewLRU(size, onEvict)
	if err != nil {
		return nil, err
	}

	return &CachingAuthenticator{
		delegate:    delegate,
		expiration:  expiration,
		lru:         lru,
		mu:          &sync.Mutex{},
		shouldCache: shouldCache,
	}, nil
}

func (c CachingAuthenticator) ValidateAndSetAuthData(token string, ec echo.Context) (bool, error) {
	entry := c.getCachedEntry(token)
	if entry != nil {
		SetAuthData(ec, entry.auth)
		return true, nil
	}

	res, err := c.delegate.ValidateAndSetAuthData(token, ec)
	auth := GetAuthData(ec.Request().Context())

	if c.shouldCache(auth) {
		entry := CacheEntry{
			token:  token,
			auth:   auth,
			expiry: time.Now().Add(c.expiration),
		}
		c.setCacheEntry(entry)
	}

	return res, err
}

func (c *CachingAuthenticator) getCachedEntry(token string) *CacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.lru.Get(token); ok {
		entry := e.(CacheEntry)
		if entry.IsExpired() {
			c.lru.Remove(token)
			return nil
		}
		return &entry
	}

	return nil
}

func (c *CachingAuthenticator) setCacheEntry(entry CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	_ = c.lru.Add(entry.token, entry)
}
