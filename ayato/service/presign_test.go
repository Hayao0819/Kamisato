package service_test

import (
	"bytes"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
)

func TestArchFromFilename(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{"foo-1-1-x86_64.pkg.tar.zst", "x86_64", false},
		{"bar-2-1-any.pkg.tar.zst.sig", "any", false},
		{"pkg-name-1.2.3-4-aarch64.pkg.tar.xz", "aarch64", false},
		{"repo/x86_64/foo-1-1-x86_64.pkg.tar.zst", "x86_64", false},
		{"notapackage.txt", "", true},
		{"foo.pkg.tar.zst", "", true},
		{"", "", true},
	}
	for _, tc := range tests {
		got, err := service.ArchFromFilename(tc.name)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ArchFromFilename(%q) = %q, want error", tc.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ArchFromFilename(%q): %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ArchFromFilename(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestPresignUploads(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)

	pkg := "foo-1.0-1-x86_64.pkg.tar.zst"
	sig := pkg + ".sig"
	bin.EXPECT().StoreFileWithSignedPutURL("myrepo", "x86_64", pkg).Return("https://r2/put/"+pkg, nil)
	bin.EXPECT().StoreFileWithSignedPutURL("myrepo", "x86_64", sig).Return("https://r2/put/"+sig, nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	urls, err := svc.PresignUploads("myrepo", []string{pkg, sig})
	if err != nil {
		t.Fatalf("PresignUploads: %v", err)
	}
	if urls[pkg] == "" || urls[sig] == "" {
		t.Fatalf("PresignUploads returned %v, want a URL per file", urls)
	}
}

func TestPresignUploads_RejectsDisallowedArch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	// The repo declares x86_64 only; an aarch64 package must be rejected before any presign.
	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64"}, nil).AnyTimes()
	bin.EXPECT().StoreFileWithSignedPutURL(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	_, err := svc.PresignUploads("myrepo", []string{"foo-1.0-1-aarch64.pkg.tar.zst"})
	if err == nil {
		t.Fatal("expected a disallowed arch to be rejected")
	}
}

func TestPresignUploads_UnsupportedBackend(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().StoreFileWithSignedPutURL("myrepo", "x86_64", "foo-1.0-1-x86_64.pkg.tar.zst").
		Return("", blob.ErrPresignUnsupported)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	_, err := svc.PresignUploads("myrepo", []string{"foo-1.0-1-x86_64.pkg.tar.zst"})
	if !errors.Is(err, blob.ErrPresignUnsupported) {
		t.Fatalf("PresignUploads on unsupported backend = %v, want ErrPresignUnsupported unwrapped", err)
	}
}

// TestFinalizeUploads_CleanupOnPrepareFailure proves finalize deletes the client's
// direct-PUT R2 objects when validation of the fetched bytes fails: the package
// object fetched from the store is not a valid package, so prepareUpload rejects it
// and both the package and its .sig are deleted from R2.
func TestFinalizeUploads_CleanupOnPrepareFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)

	pkg := "foo-1.0-1-x86_64.pkg.tar.zst"

	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil).AnyTimes()
	// The stored package bytes are garbage, so ReadBinaryPackage fails in prepareUpload.
	bin.EXPECT().FetchFile("myrepo", "x86_64", pkg).Return(
		stream.NewFileStream(pkg, "application/octet-stream", bufferToReadSeekCloser(bytes.NewBufferString("not a package"))), nil)
	// No signature stored: FetchFile reports absence, treated as nil SigFile.
	bin.EXPECT().FetchFile("myrepo", "x86_64", pkg+".sig").Return(nil, blob.ErrNotFound)

	var deleted []string
	bin.EXPECT().DeleteFile("myrepo", "x86_64", gomock.Any()).DoAndReturn(
		func(_, _, f string) error {
			deleted = append(deleted, f)
			return nil
		}).Times(2)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	if err := svc.FinalizeUploads("myrepo", []string{pkg}); err == nil {
		t.Fatal("expected finalize to fail on an invalid stored package")
	}
	wantPkg, wantSig := false, false
	for _, d := range deleted {
		if d == pkg {
			wantPkg = true
		}
		if d == pkg+".sig" {
			wantSig = true
		}
	}
	if !wantPkg || !wantSig {
		t.Errorf("finalize cleanup deleted = %v, want both %s and its .sig", deleted, pkg)
	}
}
