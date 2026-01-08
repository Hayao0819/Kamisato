package domain

import "github.com/Hayao0819/Kamisato/ayato/stream"

// BuildRequest represents a package build request
type BuildRequest struct {
	// PKGBUILD is the PKGBUILD file content
	PKGBUILD *stream.FileStream

	// AdditionalFiles are additional files needed for the build (patches, sources, etc.)
	AdditionalFiles []*stream.FileStream

	// Arch is the target architecture (x86_64, aarch64, etc.)
	Arch string

	// GPGKey is the GPG key ID for signing (optional)
	GPGKey string
}
