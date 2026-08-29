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
	refresh := r.URL.Query().Get("refresh")
	err := h.tmpl.ExecuteTemplate(w, "layout.html", map[string]any{
		"Apps":           apps,
		"RefreshSuccess": refresh == "success",
		"RefreshError":   refresh == "error",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
