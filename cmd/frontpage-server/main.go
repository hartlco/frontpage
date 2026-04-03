package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hartlco/frontpage/internal/config"
	"github.com/hartlco/frontpage/internal/data"
	"github.com/hartlco/frontpage/internal/handlers"
)

func main() {
	cfg := config.Load()

	store := data.NewStore(cfg.DataDir)
	store.StartSync(cfg.SyncInterval)

	templates := loadTemplates()

	mux := http.NewServeMux()

	// Static files
	staticDir := findDir("static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	// Routes
	mux.Handle("GET /{$}", handlers.NewHomeHandler(store, templates["home"]))
	mux.Handle("GET /apps/{slug}", handlers.NewAppHandler(store, templates["app"]))
	mux.Handle("GET /apps/{slug}/builds/{version}", handlers.NewBuildHandler(store, templates["build"], cfg))
	mux.Handle("GET /apps/{slug}/builds/{version}/download/{filename}", handlers.NewDownloadHandler(store))
	mux.Handle("GET /apps/{slug}/builds/{version}/manifest.plist", handlers.NewManifestHandler(store, cfg))
	mux.Handle("GET /apps/{slug}/builds/{version}/qr.png", handlers.NewQRHandler(store, cfg))
	mux.Handle("GET /apps/{slug}/appcast.xml", handlers.NewAppcastHandler(store))
	mux.Handle("GET /apps/{slug}/icon", handlers.NewIconHandler(store))

	log.Printf("frontpage listening on :%s (data: %s, base: %s)", cfg.Port, cfg.DataDir, cfg.BaseURL)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
}

func loadTemplates() map[string]*template.Template {
	tmplDir := findDir("templates")
	funcMap := template.FuncMap{
		"string": func(v any) string {
			return fmt.Sprintf("%v", v)
		},
		"platformIcon": func(p data.Platform) string {
			switch p {
			case data.PlatformIOS:
				return "iOS"
			case data.PlatformAndroid:
				return "Android"
			case data.PlatformMacOS:
				return "macOS"
			}
			return string(p)
		},
		"humanSize": func(size int64) string {
			const (
				KB = 1024
				MB = KB * 1024
				GB = MB * 1024
			)
			switch {
			case size >= GB:
				return formatFloat(float64(size)/float64(GB)) + " GB"
			case size >= MB:
				return formatFloat(float64(size)/float64(MB)) + " MB"
			case size >= KB:
				return formatFloat(float64(size)/float64(KB)) + " KB"
			default:
				return formatFloat(float64(size)) + " B"
			}
		},
	}

	layoutPath := filepath.Join(tmplDir, "layout.html")

	pages := []string{"home", "app", "build"}
	templates := make(map[string]*template.Template)
	for _, page := range pages {
		t := template.Must(
			template.New("").Funcs(funcMap).ParseFiles(
				layoutPath,
				filepath.Join(tmplDir, page+".html"),
			),
		)
		templates[page] = t
	}
	return templates
}

func formatFloat(f float64) string {
	if f == float64(int(f)) {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%.1f", f)
}

func findDir(name string) string {
	// Check relative to working directory
	if info, err := os.Stat(name); err == nil && info.IsDir() {
		return name
	}
	// Check relative to executable
	exe, _ := os.Executable()
	dir := filepath.Join(filepath.Dir(exe), name)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return name
}
