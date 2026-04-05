package handlers

import (
	"fmt"
	"net/http"

	"afrita/config"
)

// HandleNotFound serves a styled 404 error page.
func HandleNotFound(w http.ResponseWriter, r *http.Request) {
	renderErrorPage(w, http.StatusNotFound, "404", "الصفحة غير موجودة",
		"عذراً، الصفحة التي تبحث عنها غير موجودة أو تم نقلها.")
}

// HandleMethodNotAllowed serves a styled 405 error page.
func HandleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	renderErrorPage(w, http.StatusMethodNotAllowed, "405", "طريقة غير مسموحة",
		"طريقة الطلب المستخدمة غير مسموحة لهذا المورد.")
}

// RenderErrorPage renders a generic error page with custom code/title/message.
func RenderErrorPage(w http.ResponseWriter, code string, statusCode int, title, message string) {
	renderErrorPage(w, statusCode, code, title, message)
}

func renderErrorPage(w http.ResponseWriter, statusCode int, code, title, message string) {
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
	_ = tmpl.ExecuteTemplate(w, "error-page", data)
}
