// Package schema is the manifest of durable KV namespaces and well-known keys.
//
// These strings are persistence schema, not implementation details. Renaming one
// requires a data migration, so repositories, decorators, and migrations import
// them from one place instead of repeating literals.
package schema

const (
	AdminAllowlist = "allow"
	PackageFiles   = "pkgfile"
	Signers        = "signers"

	TokenDenylist   = "deny"
	SessionDenylist = "deny-session"
	ReplayCodes     = "replay"

	Devices         = "device"
	DeviceUserIndex = "deviceuc"
	SpentDevices    = "devicespent"
	LogTokens       = "logtoken"
	SpentLogTokens  = "logtokenspent"

	AURPackages = "aurpkg"
	AURBases    = "aurbase"

	MigrationMetadata = "_meta_"

	// LegacyPoolPointers and LegacyPoolObjects are retained only for the
	// layout-1 unpool migration.
	LegacyPoolPointers = "poolptr"
	LegacyPoolObjects  = "poolobj"
)

const LayoutVersionKey = "layout_version"
