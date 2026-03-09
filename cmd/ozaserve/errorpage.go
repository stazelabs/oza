package main

import (
	"net/http"
)

type errorPageData struct {
	Status     int
	StatusText string
	Icon       string
	Heading    string
	Detail     string
}

// writeErrorPage renders a styled full-page HTML error page matching the OZA design language.
func writeErrorPage(w http.ResponseWriter, r *http.Request, status int, heading, detail string) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)

	icon := "&#x26A0;&#xFE0E;" // ⚠︎
	if status == http.StatusNotFound {
		icon = "&#x2205;" // ∅
	}

	renderTemplate(w, "error.html", errorPageData{
		Status:     status,
		StatusText: http.StatusText(status),
		Icon:       icon,
		Heading:    heading,
		Detail:     detail,
	})
}

// write404 is a convenience wrapper for 404 Not Found pages.
func write404(w http.ResponseWriter, r *http.Request) {
	writeErrorPage(w, r, http.StatusNotFound,
		"Page not found",
		"The page you requested doesn\u2019t exist or has been moved.")
}

// write500 is a convenience wrapper for 500 Internal Server Error pages.
func write500(w http.ResponseWriter, r *http.Request) {
	writeErrorPage(w, r, http.StatusInternalServerError,
		"Something went wrong",
		"An internal error occurred. Please try again or return to the library.")
}
