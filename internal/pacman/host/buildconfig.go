package host

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	pacmanconf "github.com/Morganamilo/go-pacmanconf"
)

type BuildRepository struct {
	Name     string
	Servers  []string
	SigLevel []string
	Usage    []string
}

type BuildConfig struct {
	Architecture string
	Repositories []BuildRepository
	PacmanConf   []byte
}

func LoadBuildConfig(path, arch string) (*BuildConfig, error) {
	if path == "" {
		return nil, fmt.Errorf("pacman config path is required")
	}
	config, err := ParseConfig(path)
	if err != nil {
		return nil, err
	}
	return BuildConfigFromParsed(config, arch)
}

func BuildConfigFromParsed(config *pacmanconf.Config, arch string) (*BuildConfig, error) {
	if config == nil {
		return nil, fmt.Errorf("pacman config is nil")
	}
	if arch == "" {
		return nil, fmt.Errorf("target architecture is required")
	}
	if len(config.Architecture) > 0 && !slices.Contains(config.Architecture, arch) {
		return nil, fmt.Errorf("pacman config architectures %q do not include target %q", config.Architecture, arch)
	}

	repositories := make([]BuildRepository, 0, len(config.Repos))
	for _, configured := range config.Repos {
		servers := make([]string, 0, len(configured.Servers))
		for _, server := range configured.Servers {
			server = strings.ReplaceAll(server, "$repo", configured.Name)
			server = strings.ReplaceAll(server, "$arch", arch)
			servers = append(servers, server)
		}
		repositories = append(repositories, BuildRepository{
			Name:     configured.Name,
			Servers:  servers,
			SigLevel: append([]string(nil), configured.SigLevel...),
			Usage:    append([]string(nil), configured.Usage...),
		})
	}

	var rendered bytes.Buffer
	rendered.WriteString("[options]\nArchitecture = ")
	rendered.WriteString(arch)
	rendered.WriteByte('\n')
	writeConfigTokens(&rendered, "SigLevel", config.SigLevel)
	writeConfigTokens(&rendered, "LocalFileSigLevel", config.LocalFileSigLevel)
	writeConfigTokens(&rendered, "RemoteFileSigLevel", config.RemoteFileSigLevel)
	for _, repository := range repositories {
		fmt.Fprintf(&rendered, "\n[%s]\n", repository.Name)
		writeConfigTokens(&rendered, "SigLevel", repository.SigLevel)
		writeConfigTokens(&rendered, "Usage", repository.Usage)
		writeConfigValues(&rendered, "Server", repository.Servers)
	}

	return &BuildConfig{
		Architecture: arch,
		Repositories: repositories,
		PacmanConf:   rendered.Bytes(),
	}, nil
}

func writeConfigTokens(out *bytes.Buffer, name string, values []string) {
	if len(values) > 0 {
		fmt.Fprintf(out, "%s = %s\n", name, strings.Join(values, " "))
	}
}

func writeConfigValues(out *bytes.Buffer, name string, values []string) {
	for _, value := range values {
		fmt.Fprintf(out, "%s = %s\n", name, value)
	}
}
