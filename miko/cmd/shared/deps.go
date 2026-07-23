package shared

import (
	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/miko/service"
	"github.com/Hayao0819/Kamisato/pkg/httpx"
)

func ServiceDependencies(cfg *conf.MikoConfig) ([]service.ServiceOption, error) {
	httpClient := httpx.Default()
	options := []service.ServiceOption{service.WithOutboundHTTPClient(httpClient)}
	if cfg.Ayato.URL == "" {
		return options, nil
	}

	repositories, err := client.NewRepository(
		cfg.Ayato.URL,
		client.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, errors.WrapErr(err, "configure Ayato repository reader")
	}
	return append(options, service.WithRepositoryDBReader(repositories)), nil
}
