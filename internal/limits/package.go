// Package limits centralizes the package-size policy shared by ayato and miko.
package limits

const (
	// DefaultPackageBytes is used when max_size is unset.
	DefaultPackageBytes int64 = 512 << 20
	// MultipartOverheadBytes leaves room for framing and a detached signature.
	MultipartOverheadBytes int64 = 1 << 20
)

// PackageBytes resolves the configured byte limit. Non-positive means the
// secure default, rather than an unbounded request.
func PackageBytes(configured int) int64 {
	if configured > 0 {
		return int64(configured)
	}
	return DefaultPackageBytes
}

// MultipartBytes returns the request-body limit for one or more package parts.
func MultipartBytes(configured int) int64 {
	return PackageBytes(configured) + MultipartOverheadBytes
}

// Exceeds reports whether size violates the configured package limit.
func Exceeds(size int64, configured int) bool {
	return size > PackageBytes(configured)
}
