//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"transmission-proxy/conf"
	"transmission-proxy/internal/data"
	"transmission-proxy/internal/domain"
	"transmission-proxy/internal/service"
	"transmission-proxy/internal/trigger"
)

// initApp init kratos application.
func initApp(*conf.Bootstrap, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		data.ProviderSet,
		domain.ProviderSet,
		service.ProviderSet,
		trigger.ProviderSet,
		newApp,
	))
}
