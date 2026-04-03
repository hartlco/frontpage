package handlers

import (
	"net/http"

	"github.com/hartlco/frontpage/internal/config"
	"github.com/hartlco/frontpage/internal/data"
	"github.com/hartlco/frontpage/internal/plist"
)

type ManifestHandler struct {
	store *data.Store
	cfg   *config.Config
}

func NewManifestHandler(store *data.Store, cfg *config.Config) *ManifestHandler {
	return &ManifestHandler{store: store, cfg: cfg}
}

func (h *ManifestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	version := r.PathValue("version")

	app := h.store.App(slug)
	if app == nil || app.Platform != data.PlatformIOS {
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
	if build == nil {
		http.NotFound(w, r)
		return
	}

	downloadURL := h.cfg.BaseURL + build.DownloadPath()

	w.Header().Set("Content-Type", "application/xml")
	plist.WriteManifest(w, plist.ManifestParams{
		DownloadURL:   downloadURL,
		BundleID:      app.BundleID,
		BundleVersion: build.Version,
		Title:         app.Name,
	})
}
