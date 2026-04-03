package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/hartlco/frontpage/internal/data"
)

type IconHandler struct {
	store *data.Store
}

func NewIconHandler(store *data.Store) *IconHandler {
	return &IconHandler{store: store}
}

func (h *IconHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	app := h.store.App(slug)
	if app == nil || app.Icon == "" {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(app.DataDir, app.Icon)
	http.ServeFile(w, r, filePath)
}
