package sessions

import (
	"net/http"
	"time"
)

const CookieName = "elara_session"

const setCookieHeader = "Set-Cookie"

func SetSessionCookie(header http.Header, sessionID string, expiresAt time.Time, secure bool) {
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    sessionID,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		Expires:  expiresAt,
	}
	header.Add(setCookieHeader, cookie.String())
}

func ClearSessionCookie(header http.Header, secure bool) {
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    "",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	}
	header.Add(setCookieHeader, cookie.String())
}
