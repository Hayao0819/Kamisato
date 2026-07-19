package conf

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/koanf/v2"
)

// migrateMikoBuilderSource preserves precedence between legacy and builder.* keys.
func migrateMikoBuilderSource(source *koanf.Koanf) error {
	for _, alias := range []struct {
		legacy    string
		canonical string
		convert   func(any) (any, error)
	}{
		{"executor", "builder.backend", identityConfigValue},
		{"build.timeout", "builder.timeout", legacyMinutesDuration},
		{"build.image", "builder.docker.image", identityConfigValue},
		{"build.extra_repos", "builder.repositories", identityConfigValue},
		{"docker_host", "builder.docker.host", identityConfigValue},
		{"archbuild_template", "builder.devtools.archbuild_template", identityConfigValue},
	} {
		if err := migrateAlias(source, alias.legacy, alias.canonical, alias.convert); err != nil {
			return err
		}
	}

	cacheEnabled := true
	if source.Exists("cache.enabled") {
		parsed, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(source.Get("cache.enabled"))))
		if err != nil {
			return fmt.Errorf("cache.enabled: %w", err)
		}
		cacheEnabled = parsed
	}
	cacheAliases := map[string][]string{
		"cache.pacman_cache_dir": {
			"builder.docker.pacman_cache_dir",
			"builder.bwrap.pacman_cache_dir",
		},
		"cache.ccache_dir": {"builder.docker.ccache_dir"},
	}
	for legacy, canonicalPaths := range cacheAliases {
		for _, canonical := range canonicalPaths {
			if source.Exists(canonical) {
				continue
			}
			value := ""
			if cacheEnabled && source.Exists(legacy) {
				value = fmt.Sprint(source.Get(legacy))
			}
			if !cacheEnabled || source.Exists(legacy) {
				if err := source.Set(canonical, value); err != nil {
					return err
				}
			}
		}
	}
	source.Delete("cache")

	return validateDurationSource(source, "builder.timeout")
}

func migrateAlias(source *koanf.Koanf, legacy, canonical string, convert func(any) (any, error)) error {
	if !source.Exists(legacy) {
		return nil
	}
	if !source.Exists(canonical) {
		value, err := convert(source.Get(legacy))
		if err != nil {
			return fmt.Errorf("%s: %w", legacy, err)
		}
		if err := source.Set(canonical, value); err != nil {
			return err
		}
	}
	source.Delete(legacy)
	return nil
}

func identityConfigValue(value any) (any, error) {
	return value, nil
}

func legacyMinutesDuration(value any) (any, error) {
	minutes, err := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(value)), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("must be an integer number of minutes: %w", err)
	}
	const minute = int64(time.Minute)
	if minutes > math.MaxInt64/minute || minutes < math.MinInt64/minute {
		return nil, fmt.Errorf("duration is out of range")
	}
	return (time.Duration(minutes) * time.Minute).String(), nil
}

// validateDurationSource prevents mapstructure from treating unitless values as nanoseconds.
func validateDurationSource(source *koanf.Koanf, path string) error {
	if !source.Exists(path) {
		return nil
	}
	value := source.Get(path)
	if duration, ok := value.(time.Duration); ok {
		if duration < 0 {
			return fmt.Errorf("%s must not be negative", path)
		}
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must be a duration string such as \"30m\", got %T", path, value)
	}
	duration, err := time.ParseDuration(text)
	if err != nil {
		return fmt.Errorf("%s %q is not a valid duration: %w", path, text, err)
	}
	if duration < 0 {
		return fmt.Errorf("%s must not be negative", path)
	}
	return nil
}

func validateBuilderSource(source *koanf.Koanf) error {
	return validateDurationSource(source, "builder.timeout")
}
