package sparkle

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishedMacSignatureSurvivesAppcastGeneration(t *testing.T) {
	dir := t.TempDir()
	build := filepath.Join(dir, "builds", "1.2.0-9")
	if err := os.MkdirAll(build, 0o755); err != nil {
		t.Fatal(err)
	}
	const signature = "c2lnbmF0dXJlLXN0YXlzLWJvdW5kLXRvLXRoZS1wYXlsb2Fk"
	metadata, _ := json.Marshal(map[string]any{
		"version": "1.2.0", "build_number": "9", "date": "2026-10-24T12:00:00Z",
		"file": "App.dmg", "size": 1234, "sparkle_signature": signature,
	})
	if err := os.WriteFile(filepath.Join(build, "build.json"), metadata, 0o644); err != nil {
		t.Fatal(err)
	}
	var feed bytes.Buffer
	if err := GenerateAppcast(&feed, dir, "App", "https://builds.example.com", "app"); err != nil {
		t.Fatal(err)
	}
	decoder := xml.NewDecoder(&feed)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if element, ok := token.(xml.StartElement); ok && element.Name.Local == "enclosure" {
			attrs := map[xml.Name]string{}
			for _, attr := range element.Attr {
				attrs[attr.Name] = attr.Value
			}
			// Sparkle consumes this exact namespace and attribute, as emitted
			// by its sign_update tool; a different name means "unsigned".
			signatureName := xml.Name{Space: "http://www.andymatuschak.org/xml-namespaces/sparkle", Local: "edSignature"}
			if attrs[signatureName] != signature || attrs[xml.Name{Local: "length"}] != "1234" || attrs[xml.Name{Local: "url"}] != "https://builds.example.com/apps/app/builds/1.2.0-9/download/App.dmg" {
				t.Fatalf("archive signature, length and URL must remain together: %v", attrs)
			}
			return
		}
	}
	t.Fatal("generated feed has no enclosure")
}
