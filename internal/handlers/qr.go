package handlers

import (
	"net/http"

	"github.com/hartlco/frontpage/internal/config"
	"github.com/hartlco/frontpage/internal/data"
	qrcode "github.com/skip2/go-qrcode"
)

type QRHandler struct {
	store *data.Store
	cfg   *config.Config
}

func NewQRHandler(store *data.Store, cfg *config.Config) *QRHandler {
	return &QRHandler{store: store, cfg: cfg}
}

func (h *QRHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var target string
	switch app.Platform {
	case data.PlatformIOS:
		manifestURL := h.cfg.BaseURL + build.ManifestPath()
		target = "itms-services://?action=download-manifest&url=" + manifestURL
	case data.PlatformAndroid:
		target = h.cfg.BaseURL + build.DownloadPath()
	default:
		target = h.cfg.BaseURL + build.DownloadPath()
	}

	png, err := qrcode.Encode(target, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}
