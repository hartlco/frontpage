package data

import (
	"log"
	"os/exec"
	"time"
)

func (s *Store) StartSync(intervalSeconds int) {
	s.sync()

	if intervalSeconds <= 0 {
		return
	}

	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	go func() {
		for range ticker.C {
			s.sync()
		}
	}()
	log.Printf("git sync started (every %ds)", intervalSeconds)
}

func (s *Store) sync() {
	cmd := exec.Command("git", "-C", s.dataDir, "pull", "--ff-only")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("git pull failed: %v\n%s", err, output)
		return
	}

	if string(output) != "Already up to date.\n" {
		log.Printf("git pull: %s", output)
		s.Reload()
	}
}
