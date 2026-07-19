package service_test

import (
	"archive/tar"
	"bytes"
	"testing"

	"github.com/klauspost/compress/zstd"
)

type packageMember struct {
	name string
	body string
}

type packageFixture struct {
	extraPKGINFO string
	members      []packageMember
}

type packageOption func(*packageFixture)

func withPKGINFO(fields string) packageOption {
	return func(fixture *packageFixture) {
		fixture.extraPKGINFO += fields
	}
}

func withPackageMember(name, body string) packageOption {
	return func(fixture *packageFixture) {
		fixture.members = append(fixture.members, packageMember{name: name, body: body})
	}
}

func buildPackage(t *testing.T, name, version, arch string, options ...packageOption) []byte {
	t.Helper()
	fixture := packageFixture{}
	for _, option := range options {
		option(&fixture)
	}

	pkginfo := "pkgname = " + name + "\n" +
		"pkgver = " + version + "\n" +
		"arch = " + arch + "\n" +
		"xdata = pkgtype=pkg\n" +
		fixture.extraPKGINFO
	members := append([]packageMember{{name: ".PKGINFO", body: pkginfo}}, fixture.members...)

	var archive bytes.Buffer
	tarWriter := tar.NewWriter(&archive)
	for _, member := range members {
		header := &tar.Header{Name: member.name, Mode: 0o644, Size: int64(len(member.body))}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write %s header: %v", member.name, err)
		}
		if _, err := tarWriter.Write([]byte(member.body)); err != nil {
			t.Fatalf("write %s: %v", member.name, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close package archive: %v", err)
	}

	var compressed bytes.Buffer
	zstdWriter, err := zstd.NewWriter(&compressed)
	if err != nil {
		t.Fatalf("create zstd writer: %v", err)
	}
	if _, err := zstdWriter.Write(archive.Bytes()); err != nil {
		t.Fatalf("compress package archive: %v", err)
	}
	if err := zstdWriter.Close(); err != nil {
		t.Fatalf("close zstd writer: %v", err)
	}
	return compressed.Bytes()
}
