package sign

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// SignatureInfo is metadata read from a detached signature packet without
// verifying it; Fingerprint is empty when the issuer subpacket is absent.
type SignatureInfo struct {
	KeyID       string
	Fingerprint string
	CreatedAt   time.Time
	Hash        string
	PubKeyAlgo  string
}

// InspectDetached reads the first signature packet from armored or binary
// detached-signature bytes.
func InspectDetached(data []byte) (*SignatureInfo, error) {
	var reader io.Reader = bytes.NewReader(data)
	if bytes.HasPrefix(data, []byte("-----BEGIN PGP")) {
		block, err := armor.Decode(reader)
		if err != nil {
			return nil, fmt.Errorf("decode armored signature: %w", err)
		}
		reader = block.Body
	}
	packets := packet.NewReader(reader)
	for {
		p, err := packets.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("no signature packet found")
		}
		if err != nil {
			return nil, fmt.Errorf("read signature packet: %w", err)
		}
		sig, ok := p.(*packet.Signature)
		if !ok {
			continue
		}
		info := &SignatureInfo{
			CreatedAt:  sig.CreationTime,
			Hash:       sig.Hash.String(),
			PubKeyAlgo: pubKeyAlgoName(sig.PubKeyAlgo),
		}
		if sig.IssuerKeyId != nil {
			info.KeyID = fmt.Sprintf("%016X", *sig.IssuerKeyId)
		}
		if len(sig.IssuerFingerprint) > 0 {
			info.Fingerprint = strings.ToUpper(fmt.Sprintf("%X", sig.IssuerFingerprint))
		}
		return info, nil
	}
}

func pubKeyAlgoName(algo packet.PublicKeyAlgorithm) string {
	switch algo {
	case packet.PubKeyAlgoRSA, packet.PubKeyAlgoRSASignOnly:
		return "RSA"
	case packet.PubKeyAlgoDSA:
		return "DSA"
	case packet.PubKeyAlgoECDSA:
		return "ECDSA"
	case packet.PubKeyAlgoEdDSA:
		return "EdDSA"
	case packet.PubKeyAlgoEd25519:
		return "Ed25519"
	case packet.PubKeyAlgoEd448:
		return "Ed448"
	default:
		return fmt.Sprintf("unknown(%d)", algo)
	}
}
