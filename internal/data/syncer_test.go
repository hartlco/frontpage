package data

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type syncFixture struct {
	source, remote, cache string
}

func newSyncFixture(t *testing.T, payload []byte, lfs bool) syncFixture {
	t.Helper()
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("Git LFS is required for server sync")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_LFS_SKIP_SMUDGE", "1")
	root := t.TempDir()
	f := syncFixture{filepath.Join(root, "source"), filepath.Join(root, "remote.git"), filepath.Join(root, "cache")}
	gitTest(t, root, "init", "--initial-branch=main", f.source)
	gitTest(t, f.source, "config", "user.name", "Frontpage Test")
	gitTest(t, f.source, "config", "user.email", "test@example.com")
	if lfs {
		gitTest(t, f.source, "lfs", "install", "--local")
		gitTest(t, f.source, "lfs", "track", "*.bin")
	}
	f.publish(t, "1.0", "1", payload, false)
	gitTest(t, f.source, "tag", "old-release")
	gitTest(t, f.source, "branch", "archive")
	gitTest(t, root, "clone", "--bare", f.source, f.remote)
	gitTest(t, f.source, "remote", "add", "origin", "file://"+filepath.ToSlash(f.remote))
	if lfs {
		gitTest(t, f.source, "lfs", "push", "--all", "origin")
	}
	gitTest(t, root, "clone", "file://"+filepath.ToSlash(f.remote), f.cache)
	gitTest(t, f.cache, "lfs", "install", "--local")
	if lfs {
		gitTest(t, f.cache, "lfs", "fetch", "--all", "origin")
		gitTest(t, f.cache, "lfs", "checkout")
	}
	return f
}

func (f syncFixture) publish(t *testing.T, version, build string, payload []byte, push bool) {
	t.Helper()
	builds := filepath.Join(f.source, "apps", "demo", "builds")
	if err := os.RemoveAll(builds); err != nil {
		t.Fatal(err)
	}
	writeSyncFile(t, filepath.Join(f.source, "apps", "demo", "app.json"), []byte(`{"name":"Demo","slug":"demo","platform":"ios"}`))
	metadata, err := json.Marshal(Build{Version: version, BuildNumber: build, File: "artifact.bin", Size: int64(len(payload))})
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(builds, version+"-"+build)
	writeSyncFile(t, filepath.Join(directory, "build.json"), metadata)
	writeSyncFile(t, filepath.Join(directory, "artifact.bin"), payload)
	gitTest(t, f.source, "add", "-A")
	gitTest(t, f.source, "commit", "-m", "Publish "+version)
	if push {
		gitTest(t, f.source, "push", "origin", "main")
	}
}

func TestSyncCompactsExistingHistoryAndKeepsFollowingLatest(t *testing.T) {
	old := make([]byte, 1024*1024)
	if _, err := rand.Read(old); err != nil {
		t.Fatal(err)
	}
	f := newSyncFixture(t, old, false)
	oldBlob := gitTest(t, f.cache, "rev-parse", "HEAD:apps/demo/builds/1.0-1/artifact.bin")
	f.publish(t, "1.1", "2", []byte("already up to date"), true)
	gitTest(t, f.cache, "pull", "--ff-only")
	gitTest(t, f.cache, "gc", "--prune=now")
	before := syncDirectorySize(t, filepath.Join(f.cache, ".git", "objects"))
	store := NewStore(f.cache)
	if err := store.Sync(); err != nil {
		t.Fatal(err)
	}
	if got := gitTest(t, f.cache, "rev-list", "--all", "--count"); got != "1" {
		t.Fatalf("up-to-date full clone retained %s commits, want 1", got)
	}
	for _, version := range []string{"2", "3"} {
		payload := []byte("current artifact " + version)
		f.publish(t, version+".0", version, payload, true)
		if err := store.Sync(); err != nil {
			t.Fatal(err)
		}
		if got, want := gitTest(t, f.cache, "rev-parse", "HEAD"), gitTest(t, f.source, "rev-parse", "HEAD"); got != want {
			t.Fatalf("HEAD = %s, want %s", got, want)
		}
		if got := gitTest(t, f.cache, "rev-list", "--all", "--count"); got != "1" {
			t.Fatalf("retained %s commits, want 1", got)
		}
		if got := gitTest(t, f.cache, "rev-parse", "--is-shallow-repository"); got != "true" {
			t.Fatal("cache is not shallow")
		}
		if got := gitTest(t, f.cache, "for-each-ref", "--format=%(refname)"); got != "refs/heads/main\nrefs/remotes/origin/main" {
			t.Fatalf("obsolete refs retained: %s", got)
		}
		app := store.App("demo")
		if app == nil || len(app.Builds) != 1 || app.Builds[0].Version != version+".0" {
			t.Fatalf("store did not reload latest build: %+v", app)
		}
		assertSyncFile(t, filepath.Join(f.cache, "apps", "demo", "builds", version+".0-"+version, "artifact.bin"), payload)
	}
	if _, err := os.Stat(filepath.Join(f.cache, "apps", "demo", "builds", "1.0-1")); !os.IsNotExist(err) {
		t.Fatal("obsolete build remains in checkout")
	}
	if err := exec.Command("git", "-C", f.cache, "cat-file", "-e", oldBlob).Run(); err == nil {
		t.Fatal("deleted binary still exists in Git object storage")
	}
	after := syncDirectorySize(t, filepath.Join(f.cache, ".git", "objects"))
	if after >= before/2 {
		t.Fatalf("Git storage did not shrink: before=%d after=%d", before, after)
	}
	t.Logf("Git objects shrank from %d to %d bytes", before, after)
	if err := store.Sync(); err != nil {
		t.Fatalf("unchanged checkout failed to sync: %v", err)
	}
}

func TestSyncLFSRetainsCurrentPayloadAndRetriesFailedDownload(t *testing.T) {
	old, current, next := []byte("old LFS artifact"), []byte("current LFS artifact"), []byte("next LFS artifact")
	f := newSyncFixture(t, old, true)
	store := NewStore(f.cache)
	f.publish(t, "2.0", "2", current, true)
	// User fetch filters must not leave current downloads as pointer files or
	// cause prune to discard the current payload.
	gitTest(t, f.cache, "config", "lfs.fetchinclude", "does-not-match")
	gitTest(t, f.cache, "config", "lfs.fetchexclude", "*")
	if err := store.Sync(); err != nil {
		t.Fatal(err)
	}
	assertSyncFile(t, filepath.Join(f.cache, "apps/demo/builds/2.0-2/artifact.bin"), current)
	assertLFSObject(t, f.cache, old, false)
	assertLFSObject(t, f.cache, current, true)
	previous := gitTest(t, f.cache, "rev-parse", "HEAD")
	f.publish(t, "3.0", "3", next, false)
	t.Setenv("GIT_LFS_SKIP_PUSH", "1")
	gitTest(t, f.source, "push", "origin", "main")
	if err := store.Sync(); err == nil {
		t.Fatal("sync succeeded despite a missing remote LFS object")
	}
	if got := gitTest(t, f.cache, "rev-parse", "HEAD"); got != previous {
		t.Fatal("failed download changed the served revision")
	}
	if got := gitTest(t, f.cache, "rev-parse", "@{upstream}"); got != previous {
		t.Fatal("failed download changed the known upstream")
	}
	if got := store.App("demo").Builds[0].Version; got != "2.0" {
		t.Fatalf("failed download changed app listing: %s", got)
	}
	assertSyncFile(t, filepath.Join(f.cache, "apps/demo/builds/2.0-2/artifact.bin"), current)
	assertLFSObject(t, f.cache, current, true)
	t.Setenv("GIT_LFS_SKIP_PUSH", "")
	gitTest(t, f.source, "lfs", "push", "--all", "origin", "main")
	if err := store.Sync(); err != nil {
		t.Fatalf("retry failed after remote artifact was restored: %v", err)
	}
	assertSyncFile(t, filepath.Join(f.cache, "apps/demo/builds/3.0-3/artifact.bin"), next)
	assertLFSObject(t, f.cache, current, false)
	assertLFSObject(t, f.cache, next, true)
}

func TestSyncRejectsPublisherAndSharedCheckouts(t *testing.T) {
	for _, scenario := range []string{"modified", "untracked", "unpublished", "extra-branch", "stash", "shared-lfs", "shared-lfs-symlink", "shared-git", "worktree", "subdirectory"} {
		t.Run(scenario, func(t *testing.T) {
			f := newSyncFixture(t, []byte("old artifact"), false)
			gitTest(t, f.cache, "config", "user.name", "Frontpage Test")
			gitTest(t, f.cache, "config", "user.email", "test@example.com")
			dataDir := f.cache
			switch scenario {
			case "modified", "stash":
				writeSyncFile(t, filepath.Join(f.cache, "apps/demo/builds/1.0-1/artifact.bin"), []byte("local edit"))
				if scenario == "stash" {
					gitTest(t, f.cache, "stash", "push")
				}
			case "untracked":
				writeSyncFile(t, filepath.Join(f.cache, "unpublished.txt"), []byte("local file"))
			case "unpublished":
				gitTest(t, f.cache, "commit", "--allow-empty", "-m", "Unpublished work")
			case "extra-branch":
				gitTest(t, f.cache, "branch", "private")
			case "shared-lfs":
				gitTest(t, f.cache, "config", "lfs.storage", filepath.Join(t.TempDir(), "shared"))
			case "shared-lfs-symlink":
				objects := filepath.Join(f.cache, ".git", "lfs", "objects")
				if err := os.MkdirAll(filepath.Dir(objects), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.RemoveAll(objects); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), objects); err != nil {
					t.Fatal(err)
				}
			case "shared-git":
				writeSyncFile(t, filepath.Join(f.cache, ".git", "objects", "info", "alternates"), []byte(filepath.Join(f.source, ".git", "objects")+"\n"))
			case "worktree":
				gitTest(t, f.cache, "worktree", "add", "-b", "private", filepath.Join(t.TempDir(), "linked"))
			case "subdirectory":
				dataDir = filepath.Join(f.cache, "apps")
			}
			previous := gitTest(t, f.cache, "rev-parse", "HEAD")
			f.publish(t, "2.0", "2", []byte("new artifact"), true)
			if err := NewStore(dataDir).Sync(); err == nil || !strings.Contains(err.Error(), "dedicated, clean data clone") {
				t.Fatalf("unsafe checkout was not refused: %v", err)
			}
			if got := gitTest(t, f.cache, "rev-parse", "HEAD"); got != previous {
				t.Fatal("refused sync changed HEAD")
			}
			if scenario == "modified" {
				assertSyncFile(t, filepath.Join(f.cache, "apps/demo/builds/1.0-1/artifact.bin"), []byte("local edit"))
			}
		})
	}
}

func TestSyncFetchFailurePreservesServedBuild(t *testing.T) {
	payload := []byte("existing artifact")
	f := newSyncFixture(t, payload, false)
	store := NewStore(f.cache)
	previous := gitTest(t, f.cache, "rev-parse", "HEAD")
	gitTest(t, f.cache, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	if err := store.Sync(); err == nil {
		t.Fatal("sync succeeded with an unavailable remote")
	}
	if got := gitTest(t, f.cache, "rev-parse", "HEAD"); got != previous {
		t.Fatal("fetch failure changed HEAD")
	}
	assertSyncFile(t, filepath.Join(f.cache, "apps/demo/builds/1.0-1/artifact.bin"), payload)
}

func TestSyncAcceptsUnchangedLegacyBinariesUnderLFSAttributes(t *testing.T) {
	payload := []byte("legacy ordinary Git binary")
	f := newSyncFixture(t, payload, false)
	writeSyncFile(t, filepath.Join(f.source, ".gitattributes"), []byte("*.bin filter=lfs diff=lfs merge=lfs -text\n"))
	gitTest(t, f.source, "add", ".gitattributes")
	gitTest(t, f.source, "commit", "-m", "Add LFS attributes around a legacy binary")
	gitTest(t, f.source, "push", "origin", "main")
	gitTest(t, f.cache, "pull", "--ff-only")
	artifact := filepath.Join(f.cache, "apps/demo/builds/1.0-1/artifact.bin")
	changedTime := time.Now().Add(time.Hour)
	if err := os.Chtimes(artifact, changedTime, changedTime); err != nil {
		t.Fatal(err)
	}
	if got := gitTest(t, f.cache, "status", "--porcelain"); got == "" {
		t.Fatal("fixture did not trigger the legacy LFS false modification")
	}
	store := NewStore(f.cache)
	if err := store.Sync(); err != nil {
		t.Fatal(err)
	}
	assertSyncFile(t, artifact, payload)
	writeSyncFile(t, artifact, []byte("real local change"))
	if err := store.Sync(); err == nil {
		t.Fatal("real local changes to the legacy binary were not refused")
	}
	assertSyncFile(t, artifact, []byte("real local change"))
}

func TestSyncSerializesConcurrentRefreshes(t *testing.T) {
	f := newSyncFixture(t, []byte("existing artifact"), false)
	store := NewStore(f.cache)
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors <- store.Sync()
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func gitTest(t *testing.T, directory string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func writeSyncFile(t *testing.T, name string, value []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, value, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertSyncFile(t *testing.T, name string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("file %s differs: err=%v", name, err)
	}
}

func assertLFSObject(t *testing.T, root string, payload []byte, exists bool) {
	t.Helper()
	oid := fmt.Sprintf("%x", sha256.Sum256(payload))
	name := filepath.Join(root, ".git", "lfs", "objects", oid[:2], oid[2:4], oid)
	_, err := os.Stat(name)
	if exists && err != nil || !exists && !os.IsNotExist(err) {
		t.Fatalf("LFS object %s: exists=%v err=%v", oid, exists, err)
	}
}

func syncDirectorySize(t *testing.T, root string) int64 {
	t.Helper()
	var size int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return size
}
