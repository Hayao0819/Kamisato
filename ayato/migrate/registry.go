package migrate

// registry holds the migrations compiled into the binary; each migration file
// registers itself from init so the runner and CLI share one ordered set.
var registry []Migration

func register(m Migration) { registry = append(registry, m) }

// Registered returns every migration built into this binary.
func Registered() []Migration { return registry }
