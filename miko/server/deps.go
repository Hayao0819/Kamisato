package server

import (
	"net/http"
	"time"

	ayato "github.com/Hayao0819/Kamisato/ayato/client"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder/factory"
	mikoconfig "github.com/Hayao0819/Kamisato/miko/config"
	"github.com/Hayao0819/Kamisato/miko/service"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

func ServiceDependencies(cfg *mikoconfig.MikoConfig) ([]service.ServiceOption, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	options := []service.ServiceOption{
		service.WithOutboundHTTPClient(httpClient),
		service.WithBuildBackend(factory.New),
	}
	if cfg.Build.ResolveAURDeps {
		upstream := aurweb.NewAURUpstream(cfg.Build.AURRPCURL, aurweb.WithHTTPClient(httpClient))
		options = append(options, service.WithAURDependencies(upstream, service.NewRepoChecker()))
	}
	if cfg.Ayato.URL == "" {
		return options, nil
	}

	repositories, err := ayato.NewRepository(
		cfg.Ayato.URL,
		ayato.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, errors.WrapErr(err, "configure Ayato repository reader")
	}
	return append(options, service.WithRepositoryDBReader(repositories)), nil
}
