package app

import (
	"github.com/Hayao0819/Kamisato/ayato/config"
	"github.com/Hayao0819/Kamisato/ayato/middleware"
)

func middlewareSettings(value *config.AyatoConfig) middleware.Settings {
	if value == nil {
		return middleware.Settings{}
	}
	return middleware.Settings{
		CookieName:             value.Auth.CookieName(),
		PublicOrigin:           value.Auth.PublicOrigin,
		SelfOrigin:             value.Auth.SelfOrigin,
		AllowLegacySignerBasic: value.Auth.AllowLegacySignerBasic,
	}
}
