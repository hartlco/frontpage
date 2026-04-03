package handlers

import (
	"html/template"
	"net/http"

	"github.com/hartlco/frontpage/internal/data"
)

type AppHandler struct {
	store *data.Store
	tmpl  *template.Template
}

func NewAppHandler(store *data.Store, tmpl *template.Template) *AppHandler {
	return &AppHandler{store: store, tmpl: tmpl}
}

func (h *AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	app := h.store.App(slug)
	if app == nil {
		http.NotFound(w, r)
		return
	}

	err := h.tmpl.ExecuteTemplate(w, "app.html", map[string]any{
		"App": app,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
