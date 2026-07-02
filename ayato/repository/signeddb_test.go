package repository

import (
	"bytes"
	"io"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"

	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

func newTestEntity(t *testing.T) *openpgp.Entity {
	t.Helper()
	e, err := openpgp.NewEntity("db-signer", "test", "db-signer@example.com", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
	if err != nil {
		t.Fatalf("NewEntity: %v", err)
	}
	return e
}

// verifyDetached checks sig is a detached signature of payload made by entity.
func verifyDetached(t *testing.T, entity *openpgp.Entity, payload, sig []byte) {
	t.Helper()
	if _, err := openpgp.CheckDetachedSignature(openpgp.EntityList{entity}, bytes.NewReader(payload), bytes.NewReader(sig), nil); err != nil {
		t.Fatalf("detached signature does not verify: %v", err)
	}
}

func readStored(t *testing.T, mem *memStore, name string) []byte {
	t.Helper()
	f, err := mem.FetchFile("r", "x86_64", name)
	if err != nil {
		t.Fatalf("FetchFile %q: %v", name, err)
	}
	b, _ := io.ReadAll(f)
	f.Close()
	return b
}

// TestSignedDBArtifactsAtomic proves a signed publish stores the detached
// signatures beside their archives, serves the bare <repo>.db.sig / .files.sig as
// aliases (200, not 404), and that each signature verifies the archive committed
// in the SAME mutate — so a client never gets a db whose signature does not match.
func TestSignedDBArtifactsAtomic(t *testing.T) {
	signer := newTestEntity(t)
	mem := newMemStore()
	r := &binaryRepository{Store: mem, tool: repo.NewSigningNativeTool(signer)}

	if err := r.InitArch("r", "x86_64", true, nil); err != nil {
		t.Fatalf("InitArch: %v", err)
	}
	dir := t.TempDir()
	if err := r.RepoAdd("r", "x86_64", openSeek(t, makePkg(t, dir, "foo", "1.0-1", "x86_64")), nil, true, nil); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}

	for _, want := range []string{"r.db.tar.gz", "r.db.tar.gz.sig", "r.files.tar.gz", "r.files.tar.gz.sig"} {
		if !contains(mem.names("r", "x86_64"), want) {
			t.Errorf("signed artifact %q not stored; got %v", want, mem.names("r", "x86_64"))
		}
	}
	// The bare sigs are NOT stored; they are served as aliases of the archive sigs.
	for _, bare := range []string{"r.db.sig", "r.files.sig"} {
		if contains(mem.names("r", "x86_64"), bare) {
			t.Errorf("%q was stored; it must be served as an alias", bare)
		}
	}

	// Each served signature verifies its archive committed in the same mutate.
	for _, pair := range []struct{ sigAlias, archive string }{
		{"r.db.sig", "r.db.tar.gz"},
		{"r.files.sig", "r.files.tar.gz"},
	} {
		sf, err := r.FetchFile("r", "x86_64", pair.sigAlias)
		if err != nil {
			t.Fatalf("FetchFile %q (alias): %v", pair.sigAlias, err)
		}
		sigBytes, _ := io.ReadAll(sf)
		sf.Close()
		verifyDetached(t, signer, readStored(t, mem, pair.archive), sigBytes)
	}
}

// TestBackfillSignatures proves a db published unsigned gains valid signatures
// after signing is enabled, without changing its package set, and that a second
// backfill is a no-op.
func TestBackfillSignatures(t *testing.T) {
	signer := newTestEntity(t)
	mem := newMemStore()

	unsigned := &binaryRepository{Store: mem}
	if err := unsigned.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch: %v", err)
	}
	dir := t.TempDir()
	if err := unsigned.RepoAdd("r", "x86_64", openSeek(t, makePkg(t, dir, "foo", "1.0-1", "x86_64")), nil, false, nil); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	if contains(mem.names("r", "x86_64"), "r.db.tar.gz.sig") {
		t.Fatal("an unsigned publish must not produce a .sig")
	}

	signed := &binaryRepository{Store: mem, tool: repo.NewSigningNativeTool(signer)}
	if err := signed.BackfillSignatures("r", "x86_64"); err != nil {
		t.Fatalf("BackfillSignatures: %v", err)
	}
	if !contains(mem.names("r", "x86_64"), "r.db.tar.gz.sig") {
		t.Fatal("backfill did not create the db signature")
	}
	verifyDetached(t, signer, readStored(t, mem, "r.db.tar.gz"), readStored(t, mem, "r.db.tar.gz.sig"))

	// The package set is preserved by the no-op mutate.
	rr, err := signed.RemoteRepo("r", "x86_64")
	if err != nil {
		t.Fatalf("RemoteRepo: %v", err)
	}
	if len(rr.Pkgs) != 1 || rr.Pkgs[0].Name() != "foo" {
		t.Fatalf("backfill changed the package set: %v", rr.Pkgs)
	}

	// A second backfill sees the signature already present and does nothing.
	sigBefore := readStored(t, mem, "r.db.tar.gz.sig")
	if err := signed.BackfillSignatures("r", "x86_64"); err != nil {
		t.Fatalf("second BackfillSignatures: %v", err)
	}
	if !bytes.Equal(sigBefore, readStored(t, mem, "r.db.tar.gz.sig")) {
		t.Error("a no-op backfill rewrote the signature")
	}
}
