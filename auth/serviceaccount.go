package auth

import (
	"context"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/kelseyhightower/envconfig"
	"golang.org/x/oauth2/clientcredentials"
)

type ServiceAccount struct {
	UserId string
}

type ServiceAccountAuthenticator struct {
	TokenEndpoint string `envconfig:"TIDEPOOL_AUTH_SERVICE_TOKEN_ENDPOINT"`
}

func NewServiceAccountAuthenticator() (*ServiceAccountAuthenticator, error) {
	s := &ServiceAccountAuthenticator{}
	err := envconfig.Process("", s)
	return s, err
}

func (s *ServiceAccountAuthenticator) GetServiceAccount(ctx context.Context, clientId, clientSecret string) (*ServiceAccount, error) {
	if s.TokenEndpoint == "" {
		return nil, fmt.Errorf("token endpoint is missing")
	}

	config := clientcredentials.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		TokenURL:     s.TokenEndpoint,
		Scopes:       []string{"openid"},
	}

	tokens, err := config.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to obtain tokens: %w", err)
	}

	idToken, ok := tokens.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("invalid id token: %w", err)
	}

	// We just retrieved the token using the client credentials, so it's ok to not verify the token
	claims := jwt.RegisteredClaims{}
	_, _, err = jwt.NewParser().ParseUnverified(idToken, &claims)
	if err != nil {
		return nil, fmt.Errorf("unable to parse id token: %w", err)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("id token subject is missing")
	}

	return &ServiceAccount{
		UserId: claims.Subject,
	}, nil
}
