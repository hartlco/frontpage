package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/hartlco/frontpage/internal/data"
)

type AppcastHandler struct {
	store *data.Store
}

func NewAppcastHandler(store *data.Store) *AppcastHandler {
	return &AppcastHandler{store: store}
}

func (h *AppcastHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	app := h.store.App(slug)
	if app == nil || app.Platform != data.PlatformMacOS {
		http.NotFound(w, r)
		return
	}

	appcastPath := filepath.Join(app.DataDir, "appcast.xml")
	w.Header().Set("Content-Type", "application/xml")
	http.ServeFile(w, r, appcastPath)
}
