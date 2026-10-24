package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSparkleSignatureRequiresMacOSAndValidEd25519Shape(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(make([]byte, 64))
	for _, test := range []struct {
		platform, signature string
		valid               bool
	}{
		{"macos", valid, true}, {"macos", "", true}, {"ios", "", true},
		{"ios", valid, false}, {"macos", "not-base64", false},
		{"macos", base64.StdEncoding.EncodeToString(make([]byte, 32)), false},
	} {
		if err := validateSparkleSignature(test.platform, test.signature); (err == nil) != test.valid {
			t.Errorf("platform=%s signature=%q error=%v", test.platform, test.signature, err)
		}
	}
}

func TestRemoveOlderBuildsKeepsSelectedBuild(t *testing.T) {
	buildsDir := t.TempDir()
	for _, name := range []string{"1.0.0-1", "1.1.0-2", "2.0.0-3"} {
		dir := filepath.Join(buildsDir, name)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "app.ipa"), []byte("binary"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := removeOlderBuilds(buildsDir, "2.0.0-3")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"1.0.0-1", "1.1.0-2"}; !reflect.DeepEqual(removed, want) {
		t.Fatalf("removed = %v, want %v", removed, want)
	}
	if _, err := os.Stat(filepath.Join(buildsDir, "2.0.0-3", "app.ipa")); err != nil {
		t.Fatalf("kept artifact is missing: %v", err)
	}
	for _, name := range removed {
		if _, err := os.Stat(filepath.Join(buildsDir, name)); !os.IsNotExist(err) {
			t.Fatalf("older build %q still exists", name)
		}
	}
}

func TestRemoveOlderBuildsLeavesNonDirectoriesAlone(t *testing.T) {
	buildsDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(buildsDir, "latest"), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(buildsDir, ".gitkeep")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := removeOlderBuilds(buildsDir, "latest")
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none", removed)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("non-build file was removed: %v", err)
	}
}

func TestPruneLFSUsesAggressiveRemoteVerifiedCleanup(t *testing.T) {
	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	if err := os.WriteFile(gitPath, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	dataDir := filepath.Join(t.TempDir(), "data repo")
	output, err := pruneLFS(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(output)), "\n")
	want := []string{"-C", dataDir, "lfs", "prune", "--force", "--verify-remote"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("git arguments = %v, want %v", got, want)
	}
}
