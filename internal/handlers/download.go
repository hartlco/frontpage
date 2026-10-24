package handlers

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"

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
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	if strings.EqualFold(filepath.Ext(filename), ".apk") {
		// APKs are ZIP containers, but advertising application/zip makes some
		// Android browsers append .zip and prevents opening the installer.
		w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, filePath)
}
