package repository

import (
	"github.com/Hayao0819/Kamisato/ayato/repository/kv"
	"github.com/Hayao0819/Kamisato/ayato/repository/kv/schema"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

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
		return errors.NewErr("signer: empty fingerprint")
	}
	return r.kv.Set(schema.Signers, fingerprint, armoredPub, 0)
}

func (r *signerRepository) DeleteSigner(fingerprint string) error {
	if fingerprint == "" {
		return errors.NewErr("signer: empty fingerprint")
	}
	return r.kv.Delete(schema.Signers, fingerprint)
}

func (r *signerRepository) ListSigners() ([][]byte, error) {
	entries, err := r.kv.List(schema.Signers)
	if err != nil {
		return nil, errors.WrapErr(err, "signer: list")
	}
	out := make([][]byte, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Value)
	}
	return out, nil
}
