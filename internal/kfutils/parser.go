package kfutils

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/v2"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
)

func parserForExtension(ext string) (koanf.Parser, error) {
	switch strings.ToLower(ext) {
	case ".yaml", ".yml":
		return yaml.Parser(), nil
	case ".json":
		return json.Parser(), nil
	case ".toml":
		return toml.Parser(), nil
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
}
