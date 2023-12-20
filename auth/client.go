package auth

import (
	"context"
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
	externalConfig := authClient.NewExternalConfig()
	if err := externalConfig.Load(loader); err != nil {
		return nil, err
	}

	config := authClient.NewConfig()
	config.ExternalConfig = externalConfig
	config.Config = externalConfig.Config

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
