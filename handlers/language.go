package handlers

import (
	"net/http"
	"net/url"
	"strings"

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

// safeRedirectTarget guards HandleSetLanguage's redirect query param
// against open-redirect (e.g. redirect=https://evil.example, a
// protocol-relative //evil.example, or a backslash variant browsers
// normalize to one). Rather than returning the attacker-influenced string
// as-is after a check, it rebuilds a brand new URL from only the parsed
// Path/RawQuery fields, so no unvalidated byte from target ever reaches
// the redirect sink. Falls back to "/dashboard" for anything empty,
// unparsable, absolute, host-carrying, or not rooted at a single "/".
func safeRedirectTarget(target string) string {
	const fallback = "/dashboard"
	if target == "" {
		return fallback
	}
	u, err := url.Parse(target)
	if err != nil ||
		u.IsAbs() ||
		u.Host != "" ||
		u.Opaque != "" ||
		u.Path == "" ||
		u.Path[0] != '/' ||
		strings.HasPrefix(u.Path, "//") ||
		strings.ContainsAny(u.Path, "\\") {
		return fallback
	}
	safe := url.URL{Path: u.Path, RawQuery: u.RawQuery}
	return safe.String()
}
