package data

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

// Sync maintains a read-only deployment cache of the tracked remote branch.
// Only its latest commit and artifacts are retained locally. The data directory
// must be a dedicated clone, separate from the checkout used to publish builds.
func (s *Store) Sync() error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	branch, remote, remoteBranch, upstream, err := s.syncTarget()
	if err != nil {
		return err
	}

	// Keep the known upstream intact until all new artifacts are available. A
	// failed download can then be retried without mistaking the old HEAD for an
	// unpublished local commit. --refmap= disables opportunistic ref updates.
	const fetched = "refs/frontpage/latest"
	if _, err := s.git(nil, "fetch", "--depth=1", "--no-tags", "--no-recurse-submodules",
		"--no-auto-maintenance", "--refmap=", "--", remote, "+"+remoteBranch+":"+fetched); err != nil {
		return err
	}
	revision, err := s.git(nil, "rev-parse", fetched)
	if err != nil {
		return err
	}
	if _, err := s.git(nil, "-c", "lfs.fetchrecentalways=false", "lfs", "fetch", "--include=", "--exclude=", remote, revision); err != nil {
		return err
	}
	if _, err := s.git(nil, "reset", "--hard", revision); err != nil {
		return err
	}
	if _, err := s.git(nil, "-c", "lfs.fetchinclude=", "-c", "lfs.fetchexclude=", "lfs", "checkout"); err != nil {
		s.Reload()
		return err
	}
	// Reload before maintenance: a cleanup failure must not leave the in-memory
	// listing pointing at builds removed by the successful checkout.
	s.Reload()
	if _, err := s.git(nil, "update-ref", upstream, revision); err != nil {
		return err
	}
	refs, err := s.git(nil, "for-each-ref", "--format=%(refname)")
	if err != nil {
		return err
	}
	var deletions strings.Builder
	for _, ref := range strings.Fields(refs) {
		if ref != branch && ref != upstream {
			fmt.Fprintf(&deletions, "delete %s\n", ref)
		}
	}
	// Do not follow origin/HEAD while deleting it: it may point at the upstream
	// ref we are retaining.
	if _, err := s.git(strings.NewReader(deletions.String()), "update-ref", "--no-deref", "--stdin"); err != nil {
		return err
	}
	if _, err := s.git(nil, "update-ref", "--no-deref", "-d", "ORIG_HEAD"); err != nil {
		return err
	}
	if _, err := s.git(nil, "reflog", "expire", "--expire=now", "--expire-unreachable=now", "--all"); err != nil {
		return err
	}
	// --recent bypasses the age window while retaining current checkout
	// objects. --force can delete those objects in newer Git LFS versions.
	if _, err := s.git(nil, "-c", "lfs.pruneremotetocheck="+remote, "-c", "lfs.fetchinclude=", "-c", "lfs.fetchexclude=",
		"lfs", "prune", "--recent", "--verify-remote"); err != nil {
		return err
	}
	if _, err := s.git(nil, "-c", "gc.autoDetach=false", "gc", "--prune=now"); err != nil {
		return err
	}
	log.Printf("synced latest data checkout %.12s and pruned obsolete local objects", revision)
	return nil
}

// Reject publisher checkouts and shared repositories before fetching or pruning.
func (s *Store) syncTarget() (branch, remote, remoteBranch, upstream string, err error) {
	fail := func(reason string) (string, string, string, string, error) {
		return "", "", "", "", fmt.Errorf("latest-checkout sync requires a dedicated, clean data clone: %s", reason)
	}
	root, err := filepath.EvalSymlinks(s.dataDir)
	if err != nil {
		return fail(err.Error())
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return fail(err.Error())
	}
	gitDir := filepath.Join(root, ".git")
	info, err := os.Lstat(gitDir)
	if err != nil || !info.IsDir() {
		return fail("data directory must be the root of a standalone Git clone")
	}
	if _, err := os.Stat(filepath.Join(gitDir, "objects", "info", "alternates")); !os.IsNotExist(err) {
		return fail("shared Git object storage is not supported")
	}
	for _, directory := range []string{filepath.Join(gitDir, "lfs"), filepath.Join(gitDir, "lfs", "objects")} {
		info, err := os.Lstat(directory)
		if err != nil && !os.IsNotExist(err) || err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fail("shared/custom Git LFS storage is not supported")
		}
	}
	top, err := s.git(nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return fail(err.Error())
	}
	top, err = filepath.EvalSymlinks(top)
	if err != nil || top != root {
		return fail("data directory must be the repository root")
	}
	worktrees, err := s.git(nil, "worktree", "list", "--porcelain")
	if err != nil || strings.Count(worktrees, "worktree ") != 1 {
		return fail("linked worktrees are not supported")
	}
	status, err := s.git(nil, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return fail(err.Error())
	}
	if status != "" {
		// Older data repositories may contain ordinary Git binaries under LFS
		// attributes. The LFS clean filter reports these unchanged files as
		// modified. Compare those paths without the filter before refusing them.
		if err := s.checkLegacyChanges(); err != nil {
			return fail("local file changes must be committed/published or moved to another clone")
		}
	}
	branch, err = s.git(nil, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		return fail("check out a branch with a configured remote upstream")
	}
	name := strings.TrimPrefix(branch, "refs/heads/")
	remote, err = s.git(nil, "config", "--get", "branch."+name+".remote")
	if err != nil || remote == "." || remote == "" {
		return fail("branch must track a remote repository")
	}
	remoteBranch, err = s.git(nil, "config", "--get", "branch."+name+".merge")
	if err != nil || !strings.HasPrefix(remoteBranch, "refs/heads/") {
		return fail("branch must track a remote branch")
	}
	upstream, err = s.git(nil, "rev-parse", "--symbolic-full-name", "@{upstream}")
	if err != nil || !strings.HasPrefix(upstream, "refs/remotes/") {
		return fail("branch must have an existing remote-tracking ref")
	}
	unpublished, err := s.git(nil, "rev-list", "--max-count=1", "HEAD", "--not", upstream)
	if err != nil {
		return fail(err.Error())
	}
	if unpublished != "" {
		return fail("local branch has unpublished commits")
	}
	refs, err := s.git(nil, "for-each-ref", "--format=%(refname)")
	if err != nil {
		return fail(err.Error())
	}
	for _, ref := range strings.Fields(refs) {
		switch {
		case ref == branch, ref == "refs/frontpage/latest", strings.HasPrefix(ref, "refs/remotes/"), strings.HasPrefix(ref, "refs/tags/"):
		default:
			return fail("extra local branches, stashes, or private refs must be moved to another clone")
		}
	}
	lfsEnv, err := s.git(nil, "lfs", "env")
	if err != nil {
		return fail(err.Error())
	}
	mediaDir := ""
	for _, line := range strings.Split(lfsEnv, "\n") {
		if strings.HasPrefix(line, "LocalMediaDir=") {
			mediaDir = strings.TrimPrefix(line, "LocalMediaDir=")
		}
	}
	if filepath.Clean(mediaDir) != filepath.Join(gitDir, "lfs", "objects") {
		return fail("shared/custom Git LFS storage is not supported")
	}
	return branch, remote, remoteBranch, upstream, nil
}

func (s *Store) checkLegacyChanges() error {
	untracked, err := s.git(nil, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil || untracked != "" {
		return fmt.Errorf("untracked files or unreadable index")
	}
	if _, err := s.git(nil, "diff", "--cached", "--quiet"); err != nil {
		return err
	}
	modified, err := s.gitOutput(nil, "diff", "--name-only", "-z")
	if err != nil {
		return err
	}
	if modified == "" {
		return fmt.Errorf("unexpected Git status")
	}
	args := []string{"-c", "filter.lfs.process=", "-c", "filter.lfs.clean=", "-c", "filter.lfs.required=false", "diff", "--quiet", "--"}
	args = append(args, strings.Split(strings.TrimSuffix(modified, "\x00"), "\x00")...)
	_, err = s.git(nil, args...)
	return err
}

func (s *Store) git(input io.Reader, args ...string) (string, error) {
	output, err := s.gitOutput(input, args...)
	return strings.TrimSpace(output), err
}

func (s *Store) gitOutput(input io.Reader, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", s.dataDir}, args...)...)
	cmd.Stdin = input
	// Fetch LFS objects explicitly before changing files. Reset then stays
	// offline and lfs checkout materializes only the already-downloaded objects.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_LFS_SKIP_SMUDGE=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
