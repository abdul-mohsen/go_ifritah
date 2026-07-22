package handlers

import (
	"net/http"
	"net/url"

	"afrita/helpers"
	"afrita/resources"
)

// HandleSetLanguage persists the caller's UI language choice in a cookie and
// redirects back to the referring page. It is a plain GET navigation (used
// by the language-toggle link in templates/layouts/base.html) rather than an
// HTMX/JSON endpoint, since switching language re-renders the whole page.
func HandleSetLanguage(w http.ResponseWriter, r *http.Request) {
	lang := resources.LangAr
	if r.URL.Query().Get("lang") == resources.LangEn {
		lang = resources.LangEn
	}

	http.SetCookie(w, &http.Cookie{
		Name:     helpers.LangCookieName,
		Value:    lang,
		Path:     "/",
		HttpOnly: true,
		// The app runs behind a TLS-terminating proxy, so Secure=true is
		// always correct (matches every other cookie set across the app).
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   365 * 24 * 60 * 60, // 1 year
	})

	http.Redirect(w, r, safeRedirectTarget(r.URL.Query().Get("redirect")), http.StatusSeeOther)
}

// safeRedirectTarget returns target if it is a same-site relative path,
// otherwise "/dashboard". Guards HandleSetLanguage's redirect query param
// against open-redirect (e.g. redirect=https://evil.example).
func safeRedirectTarget(target string) string {
	if target == "" {
		return "/dashboard"
	}
	u, err := url.Parse(target)
	if err != nil || u.IsAbs() || u.Host != "" || len(u.Path) == 0 || u.Path[0] != '/' {
		return "/dashboard"
	}
	return target
}
