package auth

import "net/http"

const CookieName = "ofan_session"

func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	cookie := http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Secure:   r.TLS != nil,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, &cookie)
}

func ClearSessionCookie(w http.ResponseWriter) {
	cookie := http.Cookie{
		Name:   CookieName,
		Path:   "/",
		MaxAge: -1,
	}
	http.SetCookie(w, &cookie)
}
