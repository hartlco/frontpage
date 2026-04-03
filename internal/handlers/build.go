package handlers

import (
	"html/template"
	"net/http"

	"github.com/hartlco/frontpage/internal/config"
	"github.com/hartlco/frontpage/internal/data"
)

type BuildHandler struct {
	store *data.Store
	tmpl  *template.Template
	cfg   *config.Config
}

func NewBuildHandler(store *data.Store, tmpl *template.Template, cfg *config.Config) *BuildHandler {
	return &BuildHandler{store: store, tmpl: tmpl, cfg: cfg}
}

func (h *BuildHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	version := r.PathValue("version")

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
	if build == nil {
		http.NotFound(w, r)
		return
	}

	manifestURL := h.cfg.BaseURL + build.ManifestPath()
	installURL := "itms-services://?action=download-manifest&url=" + manifestURL
	downloadURL := h.cfg.BaseURL + build.DownloadPath()
	qrURL := build.QRPath()

	var qrTarget string
	switch app.Platform {
	case data.PlatformIOS:
		qrTarget = installURL
	case data.PlatformAndroid:
		qrTarget = downloadURL
	}

	err := h.tmpl.ExecuteTemplate(w, "build.html", map[string]any{
		"App":        app,
		"Build":      build,
		"InstallURL": installURL,
		"DownloadURL": downloadURL,
		"QRURL":      qrURL,
		"QRTarget":   qrTarget,
		"BaseURL":    h.cfg.BaseURL,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
