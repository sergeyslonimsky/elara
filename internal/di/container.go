package di

import (
	"context"

	"github.com/sergeyslonimsky/core/di"

	"github.com/sergeyslonimsky/elara/internal/di/config"
	"github.com/sergeyslonimsky/elara/internal/di/service"
)

type Container = di.Container[config.Config, *service.Manager]

func LoadContainer(ctx context.Context) (*Container, error) {
	return di.NewContainer[config.Config, *service.Manager](
		ctx,
		config.NewConfig,
		service.NewServiceManager,
	)
}
