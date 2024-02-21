package auth

import (
	"context"
	"github.com/kelseyhightower/envconfig"
	"github.com/tidepool-org/platform/auth"
	authClient "github.com/tidepool-org/platform/auth/client"
	"github.com/tidepool-org/platform/client"
	"github.com/tidepool-org/platform/log/null"
	"github.com/tidepool-org/platform/platform"
	"go.uber.org/fx"
)

var PlatformClientModule = fx.Provide(
	NewExternalEnvconfigLoader,
	NewEnvconfigLoader,
	NewClientEnvconfigLoader,
	NewClient,
)

func NewClient(loader authClient.ExternalConfigLoader, lifecycle fx.Lifecycle) (auth.Client, error) {
	configLoader := authClient.NewConfigLoader(loader, platform.NewEnvconfigLoader(nil))
	config := authClient.NewConfig()
	if err := config.Load(configLoader); err != nil {
		return nil, err
	}

	// The address is not automatically loaded by 'authClient.NewConfigLoader' so we have to load it manually
	// TIDEPOOL_AUTH_CLIENT_ADDRESS should be set to the url of the auth service
	// TIDEPOOL_AUTH_CLIENT_EXTERNAL_ADDRESS should be set to the base url of shoreline
	tmpCfg := &struct {
		Address string `envconfig:"TIDEPOOL_AUTH_CLIENT_ADDRESS" required:"true"`
	}{}
	if err := envconfig.Process(client.EnvconfigEmptyPrefix, tmpCfg); err != nil {
		return nil, err
	}
	config.Config.Address = tmpCfg.Address

	c, err := authClient.NewClient(config, platform.AuthorizeAsService, "clinic", null.NewLogger())
	if err != nil {
		return nil, err
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return c.Start()
		},
		OnStop: func(ctx context.Context) error {
			c.Close()
			return nil
		},
	})

	return c, nil
}

func NewClientEnvconfigLoader() client.ConfigLoader {
	return client.NewEnvconfigLoader()
}

func NewEnvconfigLoader(loader client.ConfigLoader) platform.ConfigLoader {
	return platform.NewEnvconfigLoader(loader)
}

func NewExternalEnvconfigLoader(loader platform.ConfigLoader) authClient.ExternalConfigLoader {
	return authClient.NewExternalEnvconfigLoader(loader)
}
