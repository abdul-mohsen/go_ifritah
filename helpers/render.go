package helpers

import (
	"html/template"
	"log"
	"net/http"

	"afrita/config"
)

// NotificationCountFunc is set by the notification store init() to provide
// unread notification count to templates without circular imports.
var NotificationCountFunc func() int

// Render executes a cached template by name, automatically injecting common
// data (version, user_role). For templates with a base layout it executes
// "base.html"; standalone/partial templates are executed directly.
//
// This replaces the per-request template.ParseFiles() pattern used before,
// eliminating disk I/O on every request.
func Render(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	tmpl, ok := config.Templates[name]
	if !ok || tmpl == nil {
		log.Printf("❌ Template not found in cache: %s", name)
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Auto-inject common template data
	if data == nil {
		data = make(map[string]interface{})
	}
	if _, exists := data["version"]; !exists {
		data["version"] = config.AppVersion
	}
	if _, exists := data["user_role"]; !exists {
		data["user_role"] = GetUserRole(r)
	}
	if _, exists := data["csrf_token"]; !exists {
		if c, err := r.Cookie("csrf_token"); err == nil {
			data["csrf_token"] = c.Value
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Templates that define "base.html" are layout pages; others are standalone/partials
	if tmpl.Lookup("base.html") != nil {
		if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
			log.Printf("❌ Template execution error (%s): %v", name, err)
		}
	} else {
		// Standalone: template is named "<name>.html" by ParseFiles
		templateName := name + ".html"
		if err := tmpl.ExecuteTemplate(w, templateName, data); err != nil {
			log.Printf("❌ Template execution error (%s): %v", name, err)
		}
	}
}

// RenderStandalone executes a standalone cached template (no base layout).
// Use for login, register, invoice-preview, etc.
func RenderStandalone(w http.ResponseWriter, name string, data map[string]interface{}) {
	tmpl, ok := config.Templates[name]
	if !ok || tmpl == nil {
		log.Printf("❌ Template not found in cache: %s", name)
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	if data == nil {
		data = make(map[string]interface{})
	}
	if _, exists := data["version"]; !exists {
		data["version"] = config.AppVersion
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Standalone templates are named "<name>.html" by ParseFiles.
	// We must use the explicit name to avoid non-deterministic selection
	// from component partials that are parsed alongside.
	templateName := name + ".html"
	if err := tmpl.ExecuteTemplate(w, templateName, data); err != nil {
		log.Printf("❌ Template execution error (%s): %v", name, err)
	}
}

// RenderPartial executes an HTMX partial/fragment template (no base layout, no version injection).
// Use for HTMX swap responses: vin-result, parts-results, cars-results, etc.
func RenderPartial(w http.ResponseWriter, name string, data map[string]interface{}) {
	tmpl, ok := config.Templates[name]
	if !ok || tmpl == nil {
		log.Printf("❌ Partial template not found in cache: %s", name)
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	if data == nil {
		data = make(map[string]interface{})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := executeFirstTemplate(tmpl, w, data); err != nil {
		log.Printf("❌ Partial execution error (%s): %v", name, err)
	}
}

// executeFirstTemplate finds the first non-empty named template and executes it.
// This is needed because template.New("").ParseFiles(file) creates a root template
// named "" (empty) while the actual content is in a template named after the filename.
func executeFirstTemplate(tmpl *template.Template, w http.ResponseWriter, data map[string]interface{}) error {
	for _, t := range tmpl.Templates() {
		if t.Name() != "" && t.Tree != nil && len(t.Tree.Root.Nodes) > 0 {
			return tmpl.ExecuteTemplate(w, t.Name(), data)
		}
	}
	// fallback
	return tmpl.Execute(w, data)
}
