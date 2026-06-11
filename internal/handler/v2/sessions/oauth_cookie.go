package sessions

import "net/http"

// OAuth bootstrap cookies carry the OIDC state + nonce values between
// OIDCLogin (sets) and OIDCCallback (reads). Both are short-lived,
// single-use, and scoped Path=/ so the browser delivers them to the
// ConnectRPC callback endpoint regardless of the URL prefix.
const (
	OAuthStateCookieName = "elara_oauth_state"
	OAuthNonceCookieName = "elara_oauth_nonce"
)

// SetOAuthStateCookie writes the OIDC state cookie with the same security
// posture as SetSessionCookie: HttpOnly + SameSite=Lax + Path=/. The
// `secure` flag MUST be threaded through from handler.secureCookie so
// HTTP-only dev environments don't have their state cookies silently
// rejected by the browser. Hard-coding Secure: true here was the source
// of Bug A — the OIDC callback then failed with "missing state cookie"
// on http://localhost without anyone realising the bootstrap cookie
// never arrived.
func SetOAuthStateCookie(header http.Header, value string, secure bool) {
	setOAuthCookie(header, OAuthStateCookieName, value, secure)
}

// SetOAuthNonceCookie writes the OIDC nonce cookie. See SetOAuthStateCookie
// for the security posture and the `secure` contract.
func SetOAuthNonceCookie(header http.Header, value string, secure bool) {
	setOAuthCookie(header, OAuthNonceCookieName, value, secure)
}

// setOAuthCookie is the single chokepoint for OAuth bootstrap cookie
// construction. Keeping the struct literal here means any future security
// hardening (SameSite=Strict, __Host- prefix, etc.) lands in one place.
func setOAuthCookie(header http.Header, name, value string, secure bool) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	}
	header.Add(setCookieHeader, cookie.String())
}
