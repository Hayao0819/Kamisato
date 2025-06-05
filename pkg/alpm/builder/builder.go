package builder

type Target struct {
	Arch        string
	SignKey     string
	InstallPkgs []string
}

type Builder struct {
	Name  string
	Build func(path string, target *Target) error
}

func Determine(t *Target) *Builder {
	switch t.Arch {
	case "x86_64":
		return &Extra_x86_64
	default:
		return nil
	}
}
