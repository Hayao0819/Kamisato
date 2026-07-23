package service_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

type fakeStagedUploader struct {
	objects  map[string][]byte
	intents  []blob.StagedIntent
	listErr  error
	deleted  []string
	presigns []string
}

func (f *fakeStagedUploader) PresignStagedPut(id, name string, _ int64, _ time.Duration) (string, error) {
	f.presigns = append(f.presigns, id+"/"+name)
	return "https://storage.example/staging/" + id + "/" + name, nil
}

func (f *fakeStagedUploader) FetchStaged(id, name string) (platform.File, error) {
	data, ok := f.objects[id+"/"+name]
	if !ok {
		return nil, blob.ErrNotFound
	}
	return pkgStream(name, data), nil
}

func (f *fakeStagedUploader) DeleteStaged(id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeStagedUploader) ListStagedIntents() ([]blob.StagedIntent, error) {
	return f.intents, f.listErr
}

const stagedTestID = "0123456789abcdef0123456789abcdef"

func TestPresignUpload_CapabilityAbsent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().StagedUploads().Return(nil, false)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, baseConfig(false, ""))
	_, err := svc.PresignUpload("myrepo", []domain.StagedFileRequest{{Name: uploadName}})
	if !errors.Is(err, domain.ErrNotImplemented) {
		t.Fatalf("PresignUpload without capability = %v, want ErrNotImplemented", err)
	}
}

func TestPresignUpload_ValidatesRequests(t *testing.T) {
	for _, test := range []struct {
		name  string
		files []domain.StagedFileRequest
	}{
		{name: "empty list", files: nil},
		{name: "not an artifact", files: []domain.StagedFileRequest{{Name: "evil.txt"}}},
		{name: "path in name", files: []domain.StagedFileRequest{{Name: "a/" + uploadName}}},
		{name: "duplicate name", files: []domain.StagedFileRequest{{Name: uploadName, Size: 1}, {Name: uploadName, Size: 1}}},
		{name: "missing size", files: []domain.StagedFileRequest{{Name: uploadName}}},
		{name: "oversized", files: []domain.StagedFileRequest{{Name: uploadName, Size: 11}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			bin := mocks.NewMockBinaryRepository(ctrl)
			bin.EXPECT().StagedUploads().Return(&fakeStagedUploader{}, true)
			bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)

			cfg := baseConfig(false, "")
			cfg.MaxSize = 10
			svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, cfg)
			_, err := svc.PresignUpload("myrepo", test.files)
			if !errors.Is(err, domain.ErrInvalidUpload) {
				t.Fatalf("PresignUpload(%s) = %v, want ErrInvalidUpload", test.name, err)
			}
		})
	}
}

func TestPresignUpload_UnknownRepo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().StagedUploads().Return(&fakeStagedUploader{}, true)
	bin.EXPECT().VerifyPkgRepo("nosuch").Return(errors.New("missing"))

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, baseConfig(false, ""))
	_, err := svc.PresignUpload("nosuch", []domain.StagedFileRequest{{Name: uploadName}})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("PresignUpload(unknown repo) = %v, want ErrNotFound", err)
	}
}

func TestPresignUpload_GrantsURLsAndCollectsExpiredIntents(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	staged := &fakeStagedUploader{intents: []blob.StagedIntent{
		{ID: "feedfacefeedface", ModTime: time.Now().Add(-25 * time.Hour)},
		{ID: "deadbeefdeadbeef", ModTime: time.Now().Add(-time.Hour)},
	}}
	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().StagedUploads().Return(staged, true)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, baseConfig(false, ""))
	grant, err := svc.PresignUpload("myrepo", []domain.StagedFileRequest{
		{Name: uploadName, Size: 6},
		{Name: uploadName + ".sig", Size: 3},
	})
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(grant.ID) {
		t.Fatalf("grant id = %q, want 32 hex chars", grant.ID)
	}
	if grant.TTLSeconds != 3600 {
		t.Fatalf("ttl = %d, want 3600", grant.TTLSeconds)
	}
	for _, name := range []string{uploadName, uploadName + ".sig"} {
		url, ok := grant.URLs[name]
		if !ok || !strings.Contains(url, grant.ID+"/"+name) {
			t.Fatalf("grant url for %q = %q, %v", name, url, ok)
		}
	}
	if len(staged.deleted) != 1 || staged.deleted[0] != "feedfacefeedface" {
		t.Fatalf("gc deleted = %v, want only the expired intent", staged.deleted)
	}
}

func TestPresignUpload_GCFailureDoesNotFailTheGrant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	staged := &fakeStagedUploader{listErr: errors.New("list boom")}
	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().StagedUploads().Return(staged, true)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, baseConfig(false, ""))
	if _, err := svc.PresignUpload("myrepo", []domain.StagedFileRequest{{Name: uploadName, Size: 1}}); err != nil {
		t.Fatalf("PresignUpload with failing gc = %v, want success", err)
	}
}

func TestCommitUpload_HappyPathRunsFullPipeline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	staged := &fakeStagedUploader{objects: map[string][]byte{
		stagedTestID + "/" + uploadName: buildPackage(t, "foo", "1.0-1", "x86_64"),
	}}
	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)
	bin.EXPECT().StagedUploads().Return(staged, true)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil).AnyTimes()
	bin.EXPECT().StoreFileImmutable("myrepo", "x86_64", gomock.Any()).Return(true, nil)
	bin.EXPECT().RepoAddBatch("myrepo", "x86_64", gomock.Any(), false, gomock.Nil()).Return(nil)
	name.EXPECT().StorePackageFiles("myrepo", []repository.PackageFileEntry{
		{Arch: "x86_64", Name: "foo", FileName: uploadName},
	}).Return(nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	err := svc.CommitUpload("myrepo", stagedTestID, []domain.StagedCommitEntry{{Package: uploadName}})
	if err != nil {
		t.Fatalf("CommitUpload: %v", err)
	}
	if len(staged.deleted) != 1 || staged.deleted[0] != stagedTestID {
		t.Fatalf("deleted = %v, want the committed intent", staged.deleted)
	}
}

func TestCommitUpload_ValidationFailureLeavesStagedObjects(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	staged := &fakeStagedUploader{objects: map[string][]byte{
		stagedTestID + "/" + uploadName: []byte("not a package"),
	}}
	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().StagedUploads().Return(staged, true)
	bin.EXPECT().VerifyPkgRepo("myrepo").Return(nil)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, baseConfig(false, ""))
	err := svc.CommitUpload("myrepo", stagedTestID, []domain.StagedCommitEntry{{Package: uploadName}})
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("CommitUpload(garbage) = %v, want ErrInvalidUpload", err)
	}
	if len(staged.deleted) != 0 {
		t.Fatalf("deleted = %v, want staged objects kept for gc retry", staged.deleted)
	}
}

func TestCommitUpload_MissingStagedFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().StagedUploads().Return(&fakeStagedUploader{}, true)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, baseConfig(false, ""))
	err := svc.CommitUpload("myrepo", stagedTestID, []domain.StagedCommitEntry{{Package: uploadName}})
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("CommitUpload(missing staged file) = %v, want ErrInvalidUpload", err)
	}
}

func TestCommitUpload_RejectsMalformedRequests(t *testing.T) {
	for _, test := range []struct {
		name    string
		id      string
		entries []domain.StagedCommitEntry
	}{
		{name: "uppercase id", id: "ABCDEF0123456789", entries: []domain.StagedCommitEntry{{Package: uploadName}}},
		{name: "short id", id: "abc123", entries: []domain.StagedCommitEntry{{Package: uploadName}}},
		{name: "path id", id: "../../0123456789abcdef", entries: []domain.StagedCommitEntry{{Package: uploadName}}},
		{name: "no entries", id: stagedTestID, entries: nil},
		{name: "non-artifact package", id: stagedTestID, entries: []domain.StagedCommitEntry{{Package: "evil.txt"}}},
		{name: "non-artifact signature", id: stagedTestID, entries: []domain.StagedCommitEntry{{Package: uploadName, Signature: "evil.txt"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			staged := &fakeStagedUploader{objects: map[string][]byte{}}
			bin := mocks.NewMockBinaryRepository(ctrl)
			bin.EXPECT().StagedUploads().Return(staged, true)

			svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, baseConfig(false, ""))
			err := svc.CommitUpload("myrepo", test.id, test.entries)
			if !errors.Is(err, domain.ErrInvalidUpload) {
				t.Fatalf("CommitUpload(%s) = %v, want ErrInvalidUpload", test.name, err)
			}
		})
	}
}

func TestCommitUpload_CapabilityAbsent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	bin.EXPECT().StagedUploads().Return(nil, false)

	svc := service.New(mocks.NewMockNameStore(ctrl), bin, nil, nil, baseConfig(false, ""))
	err := svc.CommitUpload("myrepo", stagedTestID, []domain.StagedCommitEntry{{Package: uploadName}})
	if !errors.Is(err, domain.ErrNotImplemented) {
		t.Fatalf("CommitUpload without capability = %v, want ErrNotImplemented", err)
	}
}
