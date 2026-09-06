package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/hartlco/frontpage/internal/sparkle"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "publish" {
		fmt.Fprintf(os.Stderr, "Usage: frontpage-cli publish [flags]\n")
		os.Exit(1)
	}

	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	dataDir := fs.String("data-dir", ".", "Path to data repo clone")
	platform := fs.String("platform", "", "Platform: ios, android, macos")
	app := fs.String("app", "", "App slug (folder name)")
	version := fs.String("version", "", "Semantic version (e.g. 1.2.0)")
	buildNum := fs.String("build", "", "Build number")
	file := fs.String("file", "", "Path to artifact file")
	notes := fs.String("notes", "", "Release notes")
	baseURL := fs.String("base-url", "", "Server base URL (e.g. https://builds.hartl.co)")
	bundleID := fs.String("bundle-id", "", "Bundle ID (for new apps)")
	name := fs.String("name", "", "Display name (for new apps, defaults to app slug)")
	minOS := fs.String("min-os", "", "Minimum OS version")
	keepLatest := fs.Bool("keep-latest", false, "Remove all older builds for this app")

	fs.Parse(os.Args[2:])

	if *platform == "" || *app == "" || *version == "" || *buildNum == "" || *file == "" || *baseURL == "" {
		fmt.Fprintf(os.Stderr, "Required flags: --platform, --app, --version, --build, --file, --base-url\n")
		fs.PrintDefaults()
		os.Exit(1)
	}

	if *platform != "ios" && *platform != "android" && *platform != "macos" {
		log.Fatalf("invalid platform: %s (must be ios, android, or macos)", *platform)
	}

	// Validate file exists
	srcInfo, err := os.Stat(*file)
	if err != nil {
		log.Fatalf("artifact file not found: %v", err)
	}

	// Create directories
	appDir := filepath.Join(*dataDir, "apps", *app)
	versionDir := filepath.Join(appDir, "builds", *version+"-"+*buildNum)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		log.Fatalf("failed to create directory: %v", err)
	}

	// Copy artifact
	filename := filepath.Base(*file)
	dstPath := filepath.Join(versionDir, filename)
	if err := copyFile(*file, dstPath); err != nil {
		log.Fatalf("failed to copy artifact: %v", err)
	}
	fmt.Printf("Copied %s -> %s\n", *file, dstPath)

	// Write build.json
	buildJSON := map[string]any{
		"version":      *version,
		"build_number": *buildNum,
		"date":         time.Now().UTC().Format(time.RFC3339),
		"notes":        *notes,
		"file":         filename,
		"size":         srcInfo.Size(),
	}
	if *minOS != "" {
		buildJSON["min_os_version"] = *minOS
	}
	if err := writeJSON(filepath.Join(versionDir, "build.json"), buildJSON); err != nil {
		log.Fatalf("failed to write build.json: %v", err)
	}
	fmt.Println("Created build.json")

	// Create app.json if it doesn't exist
	appJSONPath := filepath.Join(appDir, "app.json")
	if _, err := os.Stat(appJSONPath); os.IsNotExist(err) {
		displayName := *name
		if displayName == "" {
			displayName = *app
		}
		bid := *bundleID
		if bid == "" {
			bid = "com.example." + *app
		}
		appJSON := map[string]any{
			"name":        displayName,
			"slug":        *app,
			"platform":    *platform,
			"bundle_id":   bid,
			"description": "",
			"icon":        "",
		}
		if err := writeJSON(appJSONPath, appJSON); err != nil {
			log.Fatalf("failed to write app.json: %v", err)
		}
		fmt.Println("Created app.json")
	}

	// Remove older builds only after the new artifact and metadata have been
	// written successfully. git add -A below stages the deleted artifacts too.
	if *keepLatest {
		removed, err := removeOlderBuilds(filepath.Join(appDir, "builds"), filepath.Base(versionDir))
		if err != nil {
			log.Fatalf("failed to remove older builds: %v", err)
		}
		if len(removed) == 0 {
			fmt.Println("No older builds to remove")
		} else {
			fmt.Printf("Removed older builds: %v\n", removed)
		}
	}

	// Generate appcast.xml for macOS
	if *platform == "macos" {
		appcastPath := filepath.Join(appDir, "appcast.xml")
		f, err := os.Create(appcastPath)
		if err != nil {
			log.Fatalf("failed to create appcast.xml: %v", err)
		}

		appData, _ := os.ReadFile(appJSONPath)
		var appMeta struct {
			Name string `json:"name"`
		}
		json.Unmarshal(appData, &appMeta)
		appName := appMeta.Name
		if appName == "" {
			appName = *app
		}

		if err := sparkle.GenerateAppcast(f, appDir, appName, *baseURL, *app); err != nil {
			f.Close()
			log.Fatalf("failed to generate appcast.xml: %v", err)
		}
		f.Close()
		fmt.Println("Generated appcast.xml")
	}

	// Git commit and push
	gitAdd := exec.Command("git", "-C", *dataDir, "add", "-A")
	if output, err := gitAdd.CombinedOutput(); err != nil {
		log.Fatalf("git add failed: %v\n%s", err, output)
	}

	commitMsg := fmt.Sprintf("Publish %s %s (%s)", *app, *version, *buildNum)
	gitCommit := exec.Command("git", "-C", *dataDir, "commit", "-m", commitMsg)
	if output, err := gitCommit.CombinedOutput(); err != nil {
		log.Fatalf("git commit failed: %v\n%s", err, output)
	}
	fmt.Printf("Committed: %s\n", commitMsg)

	gitPush := exec.Command("git", "-C", *dataDir, "push")
	if output, err := gitPush.CombinedOutput(); err != nil {
		log.Fatalf("git push failed: %v\n%s", err, output)
	}
	fmt.Println("Pushed to remote")

	if *keepLatest {
		if output, err := pruneLFS(*dataDir); err != nil {
			log.Printf("warning: build was published, but the local Git LFS cache could not be pruned: %v\n%s", err, output)
		} else {
			fmt.Println("Pruned local Git LFS cache")
		}
	}
}

func removeOlderBuilds(buildsDir, keep string) ([]string, error) {
	entries, err := os.ReadDir(buildsDir)
	if err != nil {
		return nil, err
	}

	var removed []string
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == keep {
			continue
		}
		if err := os.RemoveAll(filepath.Join(buildsDir, entry.Name())); err != nil {
			return removed, err
		}
		removed = append(removed, entry.Name())
	}
	sort.Strings(removed)
	return removed, nil
}

func pruneLFS(dataDir string) ([]byte, error) {
	// The push must complete first so Git LFS knows the removed payloads have a
	// remote copy. --force bypasses the recent-object retention window, while
	// --verify-remote checks the remote before deleting reachable local objects.
	cmd := exec.Command("git", "-C", dataDir, "lfs", "prune", "--force", "--verify-remote")
	return cmd.CombinedOutput()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
