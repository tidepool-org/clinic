package xealth

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/xealth_client"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"net/http"
	"sync"
	"time"
)

const gracePeriod = time.Second * 30

type authenticator struct {
	config *clientcredentials.Config
	mu     *sync.Mutex

	token *oauth2.Token
}

func newAuthenticator(config *ClientConfig) (*authenticator, error) {
	return &authenticator{
		config: &clientcredentials.Config{
			ClientID:     config.ClientId,
			ClientSecret: config.ClientSecret,
			TokenURL:     config.TokenUrl,
			AuthStyle:    oauth2.AuthStyleInHeader,
		},
		mu: &sync.Mutex{},
	}, nil
}

func (a *authenticator) GetToken(ctx context.Context) (*oauth2.Token, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.tokenIsValid() {
		token, err := a.config.Token(ctx)
		if err != nil {
			return nil, err
		}

		a.token = token
	}

	return a.token, nil
}

func (a *authenticator) tokenIsValid() bool {
	if a.token == nil || a.token.Expiry.Add(-gracePeriod).After(time.Now()) {
		return false
	}

	return true
}

func NewClient(config *ClientConfig) (xealth_client.ClientWithResponsesInterface, error) {
	auth, err := newAuthenticator(config)
	if err != nil {
		return nil, err
	}

	withToken := func(ctx context.Context, req *http.Request) error {
		token, e := auth.GetToken(ctx)
		if e != nil {
			return e
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", token.AccessToken))
		return nil
	}

	return xealth_client.NewClientWithResponses(config.ServerBaseUrl, xealth_client.WithRequestEditorFn(withToken))
}
