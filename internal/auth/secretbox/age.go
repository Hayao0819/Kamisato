package secretbox

import (
	"bytes"
	"io"
	"os"
	"strings"

	"filippo.io/age"

	"github.com/Hayao0819/Kamisato/internal/errors"
)

// ageHeader is the intro of the age v1 binary format; a value carrying it is
// ciphertext, anything else a pre-encryption plaintext value (see IsSealed). This
// lets encryption be enabled over an existing store without stranding old data.
const ageHeader = "age-encryption.org/v1"

// IsSealed reports whether v is an age ciphertext rather than a plaintext value
// written before encryption was enabled.
func IsSealed(v []byte) bool {
	return bytes.HasPrefix(v, []byte(ageHeader))
}

// ageBox is a SecretBox backed by an age X25519 keypair: it seals to the
// recipient and opens with the identity, both derived from one secret key so the
// same process reads back what it writes.
type ageBox struct {
	recipient age.Recipient
	identity  age.Identity
}

var _ SecretBox = (*ageBox)(nil)

// NewAgeX25519 builds a SecretBox from an age X25519 secret key
// ("AGE-SECRET-KEY-1..."). The recipient is derived from it.
func NewAgeX25519(secretKey string) (SecretBox, error) {
	id, err := age.ParseX25519Identity(strings.TrimSpace(secretKey))
	if err != nil {
		return nil, errors.WrapErr(err, "secretbox: parse age identity")
	}
	return &ageBox{recipient: id.Recipient(), identity: id}, nil
}

// LoadAgeIdentity resolves an age secret key from the literal value, else the file
// at path (age CLI format: comments and blank lines ignored, first key line used).
// It returns "" with no error when neither is set, so an unconfigured deployment
// falls through to plaintext.
func LoadAgeIdentity(value, path string) (string, error) {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value), nil
	}
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errors.WrapErr(err, "secretbox: read age identity file")
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line, nil
	}
	return "", errors.NewErr("secretbox: age identity file contains no key")
}

func (b *ageBox) Seal(plaintext []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, b.recipient)
	if err != nil {
		return nil, errors.WrapErr(err, "secretbox: seal")
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, errors.WrapErr(err, "secretbox: seal write")
	}
	// Close flushes the age stream; a partial write would otherwise not decrypt.
	if err := w.Close(); err != nil {
		return nil, errors.WrapErr(err, "secretbox: seal close")
	}
	return buf.Bytes(), nil
}

func (b *ageBox) Open(ciphertext []byte) ([]byte, error) {
	r, err := age.Decrypt(bytes.NewReader(ciphertext), b.identity)
	if err != nil {
		return nil, errors.WrapErr(err, "secretbox: open")
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.WrapErr(err, "secretbox: open read")
	}
	return out, nil
}
