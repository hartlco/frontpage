package data

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type Store struct {
	mu      sync.RWMutex
	apps    map[string]*App
	dataDir string
}

func NewStore(dataDir string) *Store {
	s := &Store{
		apps:    make(map[string]*App),
		dataDir: dataDir,
	}
	s.Reload()
	return s
}

func (s *Store) Reload() {
	apps := make(map[string]*App)

	appsDir := filepath.Join(s.dataDir, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		log.Printf("failed to read apps directory: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		appDir := filepath.Join(appsDir, entry.Name())
		app, err := loadApp(appDir)
		if err != nil {
			log.Printf("failed to load app %s: %v", entry.Name(), err)
			continue
		}
		apps[app.Slug] = app
	}

	s.mu.Lock()
	s.apps = apps
	s.mu.Unlock()
	log.Printf("loaded %d apps", len(apps))
}

func (s *Store) Apps() []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*App, 0, len(s.apps))
	for _, app := range s.apps {
		result = append(result, app)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (s *Store) App(slug string) *App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.apps[slug]
}

func (s *Store) DataDir() string {
	return s.dataDir
}

func loadApp(dir string) (*App, error) {
	data, err := os.ReadFile(filepath.Join(dir, "app.json"))
	if err != nil {
		return nil, err
	}

	var app App
	if err := json.Unmarshal(data, &app); err != nil {
		return nil, err
	}
	app.DataDir = dir

	buildsDir := filepath.Join(dir, "builds")
	buildEntries, err := os.ReadDir(buildsDir)
	if err != nil {
		return &app, nil
	}

	for _, entry := range buildEntries {
		if !entry.IsDir() {
			continue
		}
		build, err := loadBuild(filepath.Join(buildsDir, entry.Name()))
		if err != nil {
			log.Printf("failed to load build %s/%s: %v", app.Slug, entry.Name(), err)
			continue
		}
		build.VersionString = entry.Name()
		build.App = &app
		app.Builds = append(app.Builds, build)
	}

	sort.Slice(app.Builds, func(i, j int) bool {
		return app.Builds[i].Date.After(app.Builds[j].Date)
	})

	return &app, nil
}

func loadBuild(dir string) (*Build, error) {
	data, err := os.ReadFile(filepath.Join(dir, "build.json"))
	if err != nil {
		return nil, err
	}

	var build Build
	if err := json.Unmarshal(data, &build); err != nil {
		return nil, err
	}
	return &build, nil
}
