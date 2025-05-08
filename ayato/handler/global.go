package handler

import (
	"log"

	"github.com/Hayao0819/Kamisato/conf"
)

var config *conf.AyatoConfig

func init() {
	var err error
	config, err = conf.LoadAyatoConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
}
