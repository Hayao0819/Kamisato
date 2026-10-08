package cmd

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	ayatostore "github.com/Hayao0819/Kamisato/ayato/client/auth/store"
	"github.com/Hayao0819/Kamisato/internal/errors"
	miko "github.com/Hayao0819/Kamisato/miko/client"
	thomaconfig "github.com/Hayao0819/Kamisato/thoma/config"
	"github.com/Hayao0819/Kamisato/thoma/service/build"
)

// remoteBuild owns configuration, credentials and process signals. The build
// workflow receives the selected client and never discovers credentials itself.
func remoteBuild(parent context.Context, stdout, stderr io.Writer, options build.Options) error {
	cfg, err := thomaconfig.LoadThomaConfig(nil)
	if err != nil {
		return err
	}
	base, client, err := configuredBuildClient(cfg)
	if err != nil {
		return err
	}
	cfg.Server = base
	cfg.Makepkg = realMakepkg(cfg.Makepkg)
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return build.Run(ctx, cfg, client, stdout, stderr, options)
}

func configuredBuildClient(cfg *thomaconfig.ThomaConfig) (string, *miko.Client, error) {
	if cfg.Direct() {
		if cfg.Server == "" {
			return "", nil, errors.NewErr("direct mode needs THOMA_SERVER set to the miko URL")
		}
		client, err := miko.New(cfg.Server, cfg.ApiKey)
		return cfg.Server, client, err
	}
	endpoint, err := ayatostore.Resolve(cfg.Server)
	if err != nil {
		if errors.Is(err, ayatostore.ErrNoServerSpecified) {
			return "", nil, errors.NewErr("no ayato server configured; set THOMA_SERVER or run 'ayaka server login'")
		}
		return "", nil, err
	}
	client, err := ayatostore.NewStoredClient(endpoint)
	if err != nil {
		return "", nil, err
	}
	return endpoint.URL, client.Client, nil
}
