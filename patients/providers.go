package patients

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/go-common/clients/disc"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"go.uber.org/fx"
)

type DependenciesConfig struct {
	ShorelineHost  string `envconfig:"TIDEPOOL_SHORELINE_CLIENT_ADDRESS" default:"http://shoreline:9107"`
	SeagullHost    string `envconfig:"TIDEPOOL_SEAGULL_CLIENT_ADDRESS" default:"http://seagull:9120"`
	GatekeeperHost string `envconfig:"TIDEPOOL_PERMISSION_CLIENT_ADDRESS" default:"http://gatekeeper:9123"`
	DataHost       string `envconfig:"TIDEPOOL_DATA_CLIENT_ADDRESS" default:"http://data:9220"`
	ServerSecret   string `envconfig:"TIDEPOOL_SERVER_TOKEN"`
}

func configProvider() (DependenciesConfig, error) {
	cfg := DependenciesConfig{}
	err := envconfig.Process("", &cfg)
	return cfg, err
}

func httpClientProvider() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &http.Client{Transport: tr}
}

func shorelineProvider(config DependenciesConfig, httpClient *http.Client, lifecycle fx.Lifecycle) shoreline.Client {
	client := shoreline.NewShorelineClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.ShorelineHost)).
		WithHttpClient(httpClient).
		WithName("clinics").
		WithSecret(config.ServerSecret).
		WithTokenRefreshInterval(time.Hour).
		Build()

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return client.Start()
		},
		OnStop: func(ctx context.Context) error {
			client.Close()
			return nil
		},
	})

	return client
}

func gatekeeperProvider(config DependenciesConfig, shoreline shoreline.Client, httpClient *http.Client) clients.Gatekeeper {
	return clients.NewGatekeeperClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.GatekeeperHost)).
		WithHttpClient(httpClient).
		WithTokenProvider(shoreline).
		Build()
}

func seagullProvider(config DependenciesConfig, httpClient *http.Client) clients.Seagull {
	return clients.NewSeagullClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.SeagullHost)).
		WithHttpClient(httpClient).
		Build()
}

func dataProvider(config DependenciesConfig, shoreline shoreline.Client, httpClient *http.Client) clients.DataClient {
	return *clients.NewDataClientBuilder().
		WithHostGetter(disc.NewStaticHostGetterFromString(config.DataHost)).
		WithHttpClient(httpClient).
		WithTokenProvider(shoreline).
		Build()
}
