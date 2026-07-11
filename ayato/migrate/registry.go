package migrate

// registry is populated from each migration file's init.
var registry []Migration

func register(m Migration) { registry = append(registry, m) }

func Registered() []Migration { return registry }
