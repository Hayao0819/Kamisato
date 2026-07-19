package auth

import (
	"net/http"
	"strings"
)

// RevocationChecker is the read-only denylist capability needed while resolving
// an authenticated request.
type RevocationChecker interface {
	IsRevoked(jti string) (bool, error)
	IsSessionRevoked(sessionID string) (bool, error)
}

type CredentialSource string

const (
	ViaSession CredentialSource = "session"
	ViaBearer  CredentialSource = "bearer"
	ViaBasic   CredentialSource = "basic"
)

type Identity struct {
	GitHubID int64
	Login    string
	Via      CredentialSource
}

// RequestResolver applies token type pinning and revocation consistently for
// every HTTP surface that accepts Ayato credentials.
type RequestResolver struct {
	signer            *Signer
	revocations       RevocationChecker
	sessionCookieName string
}

func NewRequestResolver(
	signer *Signer,
	revocations RevocationChecker,
	sessionCookieName string,
) RequestResolver {
	return RequestResolver{
		signer:            signer,
		revocations:       revocations,
		sessionCookieName: sessionCookieName,
	}
}

func (r RequestResolver) Resolve(
	request *http.Request,
	allowBasic bool,
) (Identity, bool, error) {
	if request == nil || r.signer == nil {
		return Identity{}, false, nil
	}
	if r.sessionCookieName != "" {
		if cookie, err := request.Cookie(r.sessionCookieName); err == nil && cookie.Value != "" {
			identity, ok, resolveErr := r.verify(credential{
				token:      cookie.Value,
				source:     ViaSession,
				tokenTypes: []string{TypSession},
			})
			if resolveErr != nil {
				return Identity{}, false, resolveErr
			}
			if ok {
				return identity, true, nil
			}
		}
	}

	access, ok := accessCredential(request, allowBasic)
	if !ok {
		return Identity{}, false, nil
	}
	return r.verify(access)
}

// ExpiredAccessToken reports whether an access credential is signed, correctly
// typed and live in the denylist, but past its expiry.
func (r RequestResolver) ExpiredAccessToken(
	request *http.Request,
	allowBasic bool,
) (bool, error) {
	if request == nil || r.signer == nil {
		return false, nil
	}
	access, ok := accessCredential(request, allowBasic)
	if !ok {
		return false, nil
	}
	for _, tokenType := range access.tokenTypes {
		claims, expired, err := r.signer.VerifyTypAllowExpired(access.token, tokenType)
		if err != nil {
			continue
		}
		revoked, revokeErr := ClaimsRevoked(r.revocations, claims)
		if revokeErr != nil {
			return false, revokeErr
		}
		return expired && !revoked, nil
	}
	return false, nil
}

type credential struct {
	token      string
	source     CredentialSource
	tokenTypes []string
}

func (r RequestResolver) verify(value credential) (Identity, bool, error) {
	for _, tokenType := range value.tokenTypes {
		claims, err := r.signer.VerifyTyp(value.token, tokenType)
		if err != nil {
			continue
		}
		revoked, revokeErr := ClaimsRevoked(r.revocations, claims)
		if revokeErr != nil {
			return Identity{}, false, revokeErr
		}
		if revoked {
			return Identity{}, false, nil
		}
		return Identity{
			GitHubID: claims.GitHubID,
			Login:    claims.Login,
			Via:      value.source,
		}, true, nil
	}
	return Identity{}, false, nil
}

func accessCredential(request *http.Request, allowBasic bool) (credential, bool) {
	if token, ok := BearerToken(request.Header); ok {
		return credential{
			token:      token,
			source:     ViaBearer,
			tokenTypes: []string{TypCLI, TypBearer},
		}, true
	}
	if allowBasic {
		if _, password, ok := request.BasicAuth(); ok {
			return credential{
				token:      password,
				source:     ViaBasic,
				tokenTypes: []string{TypCLI},
			}, true
		}
	}
	return credential{}, false
}

func BearerToken(header http.Header) (string, bool) {
	authorization := header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(authorization, "Bearer ")
	return token, token != ""
}

func ClaimsRevoked(checker RevocationChecker, claims *Claims) (bool, error) {
	if checker == nil || claims == nil {
		return false, nil
	}
	if claims.JTI != "" {
		revoked, err := checker.IsRevoked(claims.JTI)
		if err != nil || revoked {
			return revoked, err
		}
	}
	if claims.SessionID != "" {
		return checker.IsSessionRevoked(claims.SessionID)
	}
	return false, nil
}
