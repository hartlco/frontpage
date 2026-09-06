package handlers

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hartlco/frontpage/internal/config"
	"github.com/hartlco/frontpage/internal/data"
)

func TestBuildHandlerRendersIOSInstallURL(t *testing.T) {
	dataDir := t.TempDir()
	buildDir := filepath.Join(dataDir, "apps", "demo-ios", "builds", "1.0.0-1")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "apps", "demo-ios", "app.json"), []byte(`{
  "name": "Demo iOS",
  "slug": "demo-ios",
  "platform": "ios",
  "bundle_id": "com.example.demo"
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "build.json"), []byte(`{
  "version": "1.0.0",
  "build_number": "1",
  "date": "2026-09-06T10:00:00Z",
  "file": "Demo.ipa",
  "size": 123
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	store := data.NewStore(dataDir)
	tmpl := template.Must(template.New("layout.html").Parse(`<a href="{{.InstallURL}}">Install</a>`))
	handler := NewBuildHandler(store, tmpl, &config.Config{BaseURL: "https://builds.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/apps/demo-ios/builds/1.0.0-1", nil)
	req.SetPathValue("slug", "demo-ios")
	req.SetPathValue("version", "1.0.0-1")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "#ZgotmplZ") {
		t.Fatalf("install URL was blocked by html/template: %s", body)
	}
	want := "itms-services://?action=download-manifest&amp;url=https://builds.example.com/apps/demo-ios/builds/1.0.0-1/manifest.plist"
	if !strings.Contains(body, want) {
		t.Fatalf("body = %q, want it to contain %q", body, want)
	}
}
