package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

type accessCredential struct {
	token      string
	via        string
	tokenTypes []string
}

func (m *Middleware) resolveRequester(
	c *gin.Context,
	allowBasic bool,
) (requester, bool, error) {
	if session, err := c.Cookie(m.sessionCookieName()); err == nil && session != "" {
		identity, ok, resolveErr := m.verifyCredential(accessCredential{
			token:      session,
			via:        ctxViaSession,
			tokenTypes: []string{auth.TypSession},
		})
		if resolveErr != nil {
			return requester{}, false, resolveErr
		}
		if ok {
			return identity, true, nil
		}
	}

	credential, ok := accessCredentialFromRequest(c, allowBasic)
	if !ok {
		return requester{}, false, nil
	}
	return m.verifyCredential(credential)
}

func (m *Middleware) verifyCredential(
	credential accessCredential,
) (requester, bool, error) {
	for _, tokenType := range credential.tokenTypes {
		claims, err := m.signer.VerifyTyp(credential.token, tokenType)
		if err != nil {
			continue
		}
		revoked, revokeErr := m.revoked(claims)
		if revokeErr != nil {
			return requester{}, false, revokeErr
		}
		if revoked {
			return requester{}, false, nil
		}
		return requester{
			gitHubID: claims.GitHubID,
			login:    claims.Login,
			via:      credential.via,
		}, true, nil
	}
	return requester{}, false, nil
}

func accessCredentialFromRequest(c *gin.Context, allowBasic bool) (accessCredential, bool) {
	authorization := c.GetHeader("Authorization")
	if strings.HasPrefix(authorization, "Bearer ") {
		return accessCredential{
			token:      strings.TrimPrefix(authorization, "Bearer "),
			via:        ctxViaBearer,
			tokenTypes: []string{auth.TypCLI, auth.TypBearer},
		}, true
	}
	if allowBasic {
		if _, password, ok := c.Request.BasicAuth(); ok {
			return accessCredential{
				token:      password,
				via:        ctxViaBasic,
				tokenTypes: []string{auth.TypCLI},
			}, true
		}
	}
	return accessCredential{}, false
}

// revoked checks both a concrete token and its refresh-token family.
func (m *Middleware) revoked(claims *auth.Claims) (bool, error) {
	if m.denylist == nil {
		return false, nil
	}
	if claims.JTI != "" {
		revoked, err := m.denylist.IsRevoked(claims.JTI)
		if err != nil || revoked {
			return revoked, err
		}
	}
	if claims.SessionID != "" {
		return m.denylist.IsSessionRevoked(claims.SessionID)
	}
	return false, nil
}

// expiredAccessToken reports a signed, non-revoked access credential whose only
// verification failure is expiry.
func (m *Middleware) expiredAccessToken(c *gin.Context, allowBasic bool) (bool, error) {
	if m.signer == nil {
		return false, nil
	}
	credential, ok := accessCredentialFromRequest(c, allowBasic)
	if !ok {
		return false, nil
	}
	for _, tokenType := range credential.tokenTypes {
		claims, expired, err := m.signer.VerifyTypAllowExpired(credential.token, tokenType)
		if err != nil {
			continue
		}
		revoked, revokeErr := m.revoked(claims)
		if revokeErr != nil {
			return false, revokeErr
		}
		return expired && !revoked, nil
	}
	return false, nil
}
