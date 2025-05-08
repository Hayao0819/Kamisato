package abs

var Builders = []*Builder{}

type Target struct {
	Arch string
	Base string
}

type Builder struct {
	Name  string
	Build func(path string, target *Target) error
}

func GetBuilder(name string) *Builder {
	for _, builder := range Builders {
		if builder.Name == name {
			return builder
		}
	}
	return nil
}
