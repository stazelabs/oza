package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

var templates *template.Template

func initTemplates() {
	funcMap := template.FuncMap{
		"commaInt":         commaInt,
		"formatBytes":      formatBytes,
		"formatBytesShort": formatBytesShort,
		"safeHTML":         func(s string) template.HTML { return template.HTML(s) },
		"mul":              func(a float64, b float64) float64 { return a * b },
	}
	templates = template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"),
	)
}

// renderTemplate executes a named template and writes the result to w.
// On error, it logs the error and writes a 500 response if nothing has been sent yet.
func renderTemplate(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template %s: %v", name, err)
	}
}
