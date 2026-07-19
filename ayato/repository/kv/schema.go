package kv

// These namespace strings and keys form Ayato's durable persistence schema.
// Renaming one requires a data migration, so repositories, decorators, and
// migrations refer to this shared manifest instead of repeating literals.
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
