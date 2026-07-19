package service

import (
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/ProtonMail/go-crypto/openpgp"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

// signatureTrust owns public verification material. Signing private keys remain
// in the repository/builder adapters and never enter the service.
type signatureTrust struct {
	base                openpgp.EntityList
	masters             openpgp.EntityList
	trustedFingerprints []string
	err                 error
}

func loadSignatureTrust(config *conf.AyatoConfig) signatureTrust {
	trust := signatureTrust{
		trustedFingerprints: slices.Clone(config.Verify.TrustedKeys),
	}
	if config.Verify.Keyring != "" {
		data, err := os.ReadFile(config.Verify.Keyring)
		if err == nil {
			trust.base, err = sign.ReadEntities(data)
		}
		if err != nil {
			trust.err = fmt.Errorf("load package-signature keyring: %w", err)
			slog.Error(
				"failed to load package-signature keyring",
				"path",
				config.Verify.Keyring,
				"err",
				err,
			)
		} else {
			slog.Info(
				"package-signature verification enabled",
				"keyring",
				config.Verify.Keyring,
				"trusted_keys",
				len(config.Verify.TrustedKeys),
			)
		}
	}
	for index, armored := range config.Verify.MasterKeys {
		entities, err := sign.ReadEntities([]byte(armored))
		if err != nil {
			trust.err = fmt.Errorf("parse verify.master_keys[%d]: %w", index, err)
			slog.Error("failed to parse master key", "index", index, "err", err)
			continue
		}
		trust.masters = append(trust.masters, entities...)
	}
	hasRoot := config.Verify.Keyring != "" || len(config.Verify.MasterKeys) > 0
	if config.RequireSign && !hasRoot && trust.err == nil {
		trust.err = fmt.Errorf(
			"require_sign is enabled but no verification trust root is configured",
		)
	}
	return trust
}

// verifyKeyring combines static trust roots with master-certified worker keys.
func (s *Service) verifyKeyring() (*sign.Keyring, error) {
	entities := slices.Clone(s.verifier.base)
	trusted := slices.Clone(s.verifier.trustedFingerprints)
	if s.signerRepo != nil {
		registered, err := s.signerRepo.ListSigners()
		if err != nil {
			return nil, err
		}
		for _, armored := range registered {
			workerEntities, err := sign.ReadEntities(armored)
			if err != nil {
				slog.Warn("skipping unparseable registered signer key", "err", err)
				continue
			}
			entities = append(entities, workerEntities...)
			if len(s.verifier.trustedFingerprints) > 0 {
				trusted = append(trusted, fingerprints(workerEntities)...)
			}
		}
	}
	if len(entities) == 0 {
		return nil, nil
	}
	return sign.NewKeyring(entities, trusted), nil
}

func fingerprints(entities openpgp.EntityList) []string {
	result := make([]string, 0, len(entities))
	for _, entity := range entities {
		result = append(
			result,
			fmt.Sprintf("%X", entity.PrimaryKey.Fingerprint),
		)
	}
	return result
}

func (s *Service) UnregisterSigner(fingerprint string) error {
	if s.signerRepo == nil {
		return fmt.Errorf("signer registration is not available")
	}
	return s.signerRepo.DeleteSigner(fingerprint)
}

// RegisterSigner persists one worker key after a configured master certifies it.
func (s *Service) RegisterSigner(armoredPublicKey []byte) (string, error) {
	if s.signerRepo == nil {
		return "", fmt.Errorf("signer registration is not available")
	}
	entities, err := sign.ReadEntities(armoredPublicKey)
	if err != nil {
		return "", fmt.Errorf("parse signer key: %w", err)
	}
	if len(entities) != 1 {
		return "", fmt.Errorf("expected exactly one signer key, got %d", len(entities))
	}
	worker := entities[0]
	if len(s.verifier.masters) == 0 {
		return "", fmt.Errorf("no verify.master_keys configured to certify a worker key")
	}
	if !certifiedByAny(worker, s.verifier.masters) {
		return "", fmt.Errorf("worker key is not certified by any configured master key")
	}
	fingerprint := fmt.Sprintf("%X", worker.PrimaryKey.Fingerprint)
	if err := s.signerRepo.AddSigner(fingerprint, armoredPublicKey); err != nil {
		return "", err
	}
	slog.Info("registered worker signing key", "fingerprint", fingerprint)
	return fingerprint, nil
}

func certifiedByAny(worker *openpgp.Entity, masters openpgp.EntityList) bool {
	for _, master := range masters {
		if sign.CertifiedBy(worker, master) == nil {
			return true
		}
	}
	return false
}

func (s *Service) ListSigners() ([]string, error) {
	if s.signerRepo == nil {
		return nil, nil
	}
	registered, err := s.signerRepo.ListSigners()
	if err != nil {
		return nil, err
	}
	var result []string
	for _, armored := range registered {
		entities, err := sign.ReadEntities(armored)
		if err == nil && len(entities) > 0 {
			result = append(result, fingerprints(entities)[0])
		}
	}
	return result, nil
}
