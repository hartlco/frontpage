package data

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

func (s *Store) StartSync(intervalSeconds int) {
	if err := s.Sync(); err != nil {
		log.Printf("initial git sync failed: %v", err)
	}

	if intervalSeconds <= 0 {
		return
	}

	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	go func() {
		for range ticker.C {
			if err := s.Sync(); err != nil {
				log.Printf("scheduled git sync failed: %v", err)
			}
		}
	}()
	log.Printf("git sync started (every %ds)", intervalSeconds)
}

// Sync fetches the latest data repository commit and reloads the in-memory
// store when the working tree changes. Calls are serialized so a manual sync
// cannot overlap a scheduled one.
func (s *Store) Sync() error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	cmd := exec.Command("git", "-C", s.dataDir, "pull", "--ff-only")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull: %w: %s", err, strings.TrimSpace(string(output)))
	}

	if string(output) != "Already up to date.\n" {
		log.Printf("git pull: %s", output)
		s.Reload()
	}
	return nil
}
