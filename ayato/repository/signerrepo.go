package repository

import (
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

// signerNS holds registered worker signing keys (armored public key by uppercase
// hex fingerprint). A key lands here only after its master-certification chain is
// verified, so the verify path can trust any entity it lists.
const signerNS = "signers"

type SignerRepository interface {
	AddSigner(fingerprint string, armoredPub []byte) error
	ListSigners() ([][]byte, error)
	DeleteSigner(fingerprint string) error
}

type signerRepository struct {
	kv kv.Store
}

func NewSignerRepository(store kv.Store) SignerRepository {
	return &signerRepository{kv: store}
}

func (r *signerRepository) AddSigner(fingerprint string, armoredPub []byte) error {
	if fingerprint == "" {
		return utils.NewErr("signer: empty fingerprint")
	}
	return r.kv.Set(signerNS, fingerprint, armoredPub, 0)
}

func (r *signerRepository) DeleteSigner(fingerprint string) error {
	if fingerprint == "" {
		return utils.NewErr("signer: empty fingerprint")
	}
	return r.kv.Delete(signerNS, fingerprint)
}

func (r *signerRepository) ListSigners() ([][]byte, error) {
	entries, err := r.kv.List(signerNS)
	if err != nil {
		return nil, utils.WrapErr(err, "signer: list")
	}
	out := make([][]byte, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Value)
	}
	return out, nil
}
