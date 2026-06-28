package builder

import (
	"fmt"
	"strings"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultContainerImage = "archlinux:latest"
	defaultBuildTimeout   = 30 * time.Minute
)

func archToPlatform(arch string) (*ocispec.Platform, error) {
	switch arch {
	case "x86_64":
		return &ocispec.Platform{OS: "linux", Architecture: "amd64"}, nil
	case "aarch64":
		return &ocispec.Platform{OS: "linux", Architecture: "arm64"}, nil
	case "armv7h":
		return &ocispec.Platform{OS: "linux", Architecture: "arm", Variant: "v7"}, nil
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", arch)
	}
}

// platformString renders a Docker platform spec as "os/arch[/variant]".
func platformString(p *ocispec.Platform) string {
	s := p.OS + "/" + p.Architecture
	if p.Variant != "" {
		s += "/" + p.Variant
	}
	return s
}

// shellQuote wraps s in single quotes so it can be embedded safely in an
// `sh -c` command, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
