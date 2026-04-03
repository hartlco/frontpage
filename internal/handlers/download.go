package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/hartlco/frontpage/internal/data"
)

type DownloadHandler struct {
	store *data.Store
}

func NewDownloadHandler(store *data.Store) *DownloadHandler {
	return &DownloadHandler{store: store}
}

func (h *DownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	version := r.PathValue("version")
	filename := r.PathValue("filename")

	app := h.store.App(slug)
	if app == nil {
		http.NotFound(w, r)
		return
	}

	var build *data.Build
	for _, b := range app.Builds {
		if b.VersionString == version {
			build = b
			break
		}
	}
	if build == nil || build.File != filename {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(app.DataDir, "builds", version, filename)
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	http.ServeFile(w, r, filePath)
}
