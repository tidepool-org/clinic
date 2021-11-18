package auth

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"net/http"
)

var (
	ErrUnauthenticated            = fmt.Errorf("session token is invalid")
	AuthContextKey                = AuthKey("auth")
	TidepoolSessionTokenHeaderKey = "x-tidepool-session-token"
)

type AuthKey string

type Auth struct {
	SubjectId    string `json:"subjectId"`
	ServerAccess bool   `json:"serverAccess"`
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

func NewShorelineAuthenticator(shoreline shoreline.Client) Authenticator {
	return &ShorelineAuthenticator{shoreline: shoreline}
}

func (s *ShorelineAuthenticator) ValidateAndSetAuthData(token string, ec echo.Context) (bool, error) {
	data := s.shoreline.CheckToken(token)
	if data != nil && data.UserID != "" {
		ctx := context.WithValue(ec.Request().Context(), AuthContextKey, &Auth{
			SubjectId:    data.UserID,
			ServerAccess: data.IsServer,
		})
		ec.SetRequest(ec.Request().WithContext(ctx))
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
