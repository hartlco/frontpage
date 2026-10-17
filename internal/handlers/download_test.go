package handlers

import (
	"bytes"
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hartlco/frontpage/internal/data"
)

func TestAPKDownloadKeepsAndroidMediaTypeAndFilename(t *testing.T) {
	root := t.TempDir()
	buildDir := filepath.Join(root, "apps", "android", "builds", "1.0-1")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	filename := "Prosit Android.apk"
	for path, value := range map[string]any{
		filepath.Join(root, "apps", "android", "app.json"): data.App{Name: "Android", Slug: "android", Platform: data.PlatformAndroid},
		filepath.Join(buildDir, "build.json"):              data.Build{Version: "1.0", BuildNumber: "1", File: filename},
	} {
		value, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, value, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Deliberately use ZIP magic: relying on content sniffing produces the bug.
	body := []byte("PK\x03\x04synthetic-apk-container")
	if err := os.WriteFile(filepath.Join(buildDir, filename), body, 0o644); err != nil {
		t.Fatal(err)
	}
	handler := NewDownloadHandler(data.NewStore(root))
	for _, test := range []struct {
		method, filename, byteRange string
		status                      int
		body                        []byte
	}{
		{http.MethodGet, filename, "", http.StatusOK, body},
		{http.MethodHead, filename, "", http.StatusOK, nil},
		{http.MethodGet, filename, "bytes=0-3", http.StatusPartialContent, body[:4]},
		{http.MethodGet, "unlisted.apk", "", http.StatusNotFound, nil},
	} {
		t.Run(test.method+test.byteRange+test.filename, func(t *testing.T) {
			req := httptest.NewRequest(test.method, "/download", nil)
			req.SetPathValue("slug", "android")
			req.SetPathValue("version", "1.0-1")
			req.SetPathValue("filename", test.filename)
			if test.byteRange != "" {
				req.Header.Set("Range", test.byteRange)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d", recorder.Code, test.status)
			}
			if test.status == http.StatusNotFound {
				return
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/vnd.android.package-archive" {
				t.Fatalf("Content-Type = %q", got)
			}
			kind, params, err := mime.ParseMediaType(recorder.Header().Get("Content-Disposition"))
			if err != nil || kind != "attachment" || params["filename"] != filename {
				t.Fatalf("download filename changed: kind=%s params=%v error=%v", kind, params, err)
			}
			if !bytes.Equal(recorder.Body.Bytes(), test.body) {
				t.Fatalf("download bytes changed")
			}
			if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing nosniff header")
			}
		})
	}
}
