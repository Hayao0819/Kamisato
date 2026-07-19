package docker

import (
	"fmt"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultContainerImage = builder.DefaultDockerImage
	defaultBuildTimeout   = builder.DefaultDockerTimeout
)

func archToPlatform(arch string) (*ocispec.Platform, error) {
	switch arch {
	case "x86_64":
		return &ocispec.Platform{OS: "linux", Architecture: "amd64"}, nil
	case "aarch64":
		return &ocispec.Platform{OS: "linux", Architecture: "arm64"}, nil
	case "armv7h":
		return &ocispec.Platform{OS: "linux", Architecture: "arm", Variant: "v7"}, nil
	case "i486", "i686", "pentium4":
		// archlinux32's 32-bit x86 targets all run on the linux/386 platform (same
		// IA-32 family as x86_64, no qemu). The CARCH/-march difference between them
		// rides on TARGET_CARCH and the arch-specific image, not the platform.
		return &ocispec.Platform{OS: "linux", Architecture: "386"}, nil
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", arch)
	}
}

// archToCHOST returns the GNU host triple for a pacman CARCH. archlinux32's
// pentium4 builds with the i686 toolchain, so its triple is i686-pc-linux-gnu;
// every other arch keeps the historical <arch>-pc-linux-gnu.
func archToCHOST(arch string) string {
	if arch == "pentium4" {
		return "i686-pc-linux-gnu"
	}
	return arch + "-pc-linux-gnu"
}

// platformString renders a Docker platform spec as "os/arch[/variant]".
func platformString(p *ocispec.Platform) string {
	s := p.OS + "/" + p.Architecture
	if p.Variant != "" {
		s += "/" + p.Variant
	}
	return s
}
