package kv

// AuditForeignKeys reports foreign keys and optionally removes that exact set.
// A failed audit never reaches the destructive operation.
func AuditForeignKeys(auditor KeyAuditor, prune bool) ([]string, error) {
	foreign, err := auditor.ForeignKeys()
	if err != nil || !prune || len(foreign) == 0 {
		return foreign, err
	}
	return foreign, auditor.DeleteRawKeys(foreign)
}
