package di

import (
	"context"

	"github.com/sergeyslonimsky/core/di"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/di/service"
)

type Container = di.Container[config.Config, *service.Managers]

func LoadContainer(ctx context.Context) (*Container, error) {
	return di.NewContainer[config.Config, *service.Managers](
		ctx,
		config.NewConfig,
		service.NewServiceManager,
	)
}
