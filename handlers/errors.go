package handlers

import (
	"fmt"
	"net/http"

	"afrita/config"
	"afrita/helpers"
	"afrita/resources"
)

// HandleNotFound serves a styled 404 error page.
func HandleNotFound(w http.ResponseWriter, r *http.Request) {
	lang := helpers.GetLang(r)
	renderErrorPage(w, r, http.StatusNotFound, "404", resources.T(lang, "error.404_title"),
		resources.T(lang, "error.404_message"))
}

// HandleMethodNotAllowed serves a styled 405 error page.
func HandleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	lang := helpers.GetLang(r)
	renderErrorPage(w, r, http.StatusMethodNotAllowed, "405", resources.T(lang, "error.405_title"),
		resources.T(lang, "error.405_message"))
}

// RenderErrorPage renders a generic error page with custom code/title/message.
func RenderErrorPage(w http.ResponseWriter, r *http.Request, code string, statusCode int, title, message string) {
	renderErrorPage(w, r, statusCode, code, title, message)
}

func renderErrorPage(w http.ResponseWriter, r *http.Request, statusCode int, code, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)

	tmpl, ok := config.Templates["error-page"]
	if !ok || tmpl == nil {
		// Fallback: plain HTML
		fmt.Fprintf(w, "<h1>%s - %s</h1><p>%s</p>", code, title, message)
		return
	}

	data := map[string]interface{}{
		"code":    code,
		"title":   title,
		"message": message,
		"version": config.AppVersion,
	}
	tmpl, data = helpers.BindLangData(tmpl, r, data)
	_ = tmpl.ExecuteTemplate(w, "error-page", data)
}
