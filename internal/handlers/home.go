package handlers

import (
	"html/template"
	"net/http"

	"github.com/hartlco/frontpage/internal/data"
)

type HomeHandler struct {
	store *data.Store
	tmpl  *template.Template
}

func NewHomeHandler(store *data.Store, tmpl *template.Template) *HomeHandler {
	return &HomeHandler{store: store, tmpl: tmpl}
}

func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	apps := h.store.Apps()
	err := h.tmpl.ExecuteTemplate(w, "home.html", map[string]any{
		"Apps": apps,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
