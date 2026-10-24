# Frontpage

Self-hosted app distribution server — like HockeyApp, but simple and yours.

Serve iOS, Android, and macOS builds to your testers from a clean web UI. No database, no accounts — just a GitHub repo with your build artifacts and JSON metadata.

## Features

- **Web UI** — Browse apps, view builds, download artifacts
- **iOS OTA Install** — Generates `manifest.plist` on the fly for `itms-services://` installs
- **Android APK** — Direct download with QR code for easy install
- **macOS + Sparkle** — Serves `appcast.xml` for automatic updates via Sparkle framework
- **QR Codes** — Auto-generated QR codes for iOS and Android builds
- **Git-based storage** — Your builds live in a separate GitHub repo (with Git LFS). The server keeps only the latest checkout, syncing on a schedule or from the web UI
- **CLI tool** — `frontpage-cli publish` structures builds, generates metadata, and pushes to the data repo
- **Docker** — Single container, easy to deploy

## Architecture

```
┌──────────────┐     git push      ┌──────────────────┐   shallow fetch   ┌──────────────┐
│  Your CI /   │ ──────────────▶   │  frontpage-data   │  ◀────────────── │  Frontpage   │
│  local build │   (via CLI)       │  (GitHub repo)    │    (every 15m)   │  Server      │
└──────────────┘                   └──────────────────┘                   └──────┬───────┘
                                                                                 │
                                                                                 ▼
                                                                          ┌──────────────┐
                                                                          │  Testers     │
                                                                          │  (browser)   │
                                                                          └──────────────┘
```

## Quick Start

### Run locally

```bash
go run ./cmd/frontpage-server
# Visit http://localhost:8080
```

The server defaults to `./testdata` as the data directory, which includes demo apps for all three platforms.

### Deploy with Docker

```bash
# Create a dedicated server cache next to docker-compose.yml
git clone --depth=1 --single-branch --no-tags https://github.com/hartlco/frontpage-data.git

# Start the server
docker compose up -d
```

See [Deployment](#deployment) for full VPS setup instructions.

## Publishing Builds

Use the CLI tool to publish a new build:

```bash
# Install
go install github.com/hartlco/frontpage/cmd/frontpage-cli@latest

# Publish an iOS build
frontpage-cli publish \
  --data-dir ./frontpage-data \
  --platform ios \
  --app my-app \
  --version 1.2.0 \
  --build 87 \
  --file ./build/MyApp.ipa \
  --notes "Bug fixes and performance improvements" \
  --keep-latest \
  --base-url https://builds.example.com

# Publish a macOS build (also generates appcast.xml for Sparkle)
frontpage-cli publish \
  --data-dir ./frontpage-data \
  --platform macos \
  --app my-mac-app \
  --version 2.0.0 \
  --build 150 \
  --file ./build/MyApp.dmg \
  --notes "New features" \
  --base-url https://builds.example.com
```

### CLI Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--data-dir` | Yes | Path to cloned data repo |
| `--platform` | Yes | `ios`, `android`, or `macos` |
| `--app` | Yes | App slug (used as folder name) |
| `--version` | Yes | Semantic version (e.g. `1.2.0`) |
| `--build` | Yes | Build number |
| `--file` | Yes | Path to `.ipa`, `.apk`, or `.dmg` |
| `--base-url` | Yes | Server URL (e.g. `https://builds.example.com`) |
| `--notes` | No | Release notes |
| `--name` | No | Display name (defaults to slug, used for new apps) |
| `--bundle-id` | No | Bundle identifier (used for new apps) |
| `--min-os` | No | Minimum OS version |
| `--sparkle-signature` | No | Base64 Ed25519 signature of the final macOS update archive |
| `--keep-latest` | No | Remove older builds and prune their local Git LFS data after publishing |

`--keep-latest` keeps the build being published and deletes every other build
directory for the same app before committing. Because publishing stages with
`git add -A`, the old binaries are removed from the current Git tree and remote
branch by the same commit. After the push succeeds, the CLI runs a remote-verified
`git lfs prune --force` to remove old LFS payloads from the local `.git/lfs`
cache, reducing the total size of the checkout immediately. Existing Git history
is not rewritten.

This publishing compaction applies to the checkout passed via `--data-dir`.
Keep that publishing clone separate from the server's data directory: the
server automatically discards local history during synchronization.

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `FRONTPAGE_DATA_DIR` | `./testdata` | Path to a dedicated server clone of the data repo |
| `FRONTPAGE_BASE_URL` | `http://localhost:8080` | Public URL of the server |
| `FRONTPAGE_SYNC_INTERVAL` | `900` | Seconds between latest-checkout syncs (0 disables the timer; startup/manual sync still run) |
| `FRONTPAGE_PORT` | `8080` | HTTP listen port |

## Deployment

### Server storage

The mounted data directory is a read-only deployment cache of its configured
upstream branch. Each startup, scheduled sync, or **Fetch latest builds** request:

1. Fetches just the branch tip with `--depth=1`, without tags or other branches.
2. Downloads the current revision's LFS artifacts before changing the served files.
3. Checks out that revision and reloads the app listings.
4. Removes obsolete local tags, remote refs and reflogs, prunes old LFS payloads,
   and runs Git garbage collection to delete historical binaries from disk.

Existing full server clones are compacted automatically on their first successful
sync after upgrading. GitHub history and the separate publishing clone are not
changed. Only builds present in the current data-repo tree remain hosted; build
retention is still controlled when publishing or cleaning up that tree.

Use a standalone, clean clone with one local branch tracking a remote branch.
Local edits, untracked files, unpublished commits, extra local branches, stashes,
linked worktrees and shared/custom LFS storage cause sync to stop before fetching
or pruning. Local tags and other remote-tracking refs are disposable in this
server cache. Do not run publishing or other Git commands in it while Frontpage
is running.

The remaining storage includes the working files, Git objects for the current
commit, and LFS payloads needed by that checkout. Historical build binaries no
longer accumulate in the server's Git history or LFS cache. A failed Git fetch or
LFS download leaves the previously served checkout available for retry.

### Prerequisites

- Ubuntu VPS with Docker
- A domain pointing to the VPS (e.g. `builds.example.com`)
- [Caddy](https://caddyserver.com/) for automatic HTTPS (required for iOS OTA)

### Setup

1. **Clone and start:**

```bash
mkdir -p /srv/frontpage && cd /srv/frontpage
git clone --depth=1 --single-branch --no-tags https://github.com/hartlco/frontpage-data.git
# Download docker-compose.yml, then:
docker compose up -d
```

The data repo is public, so no SSH keys are needed — the server pulls over HTTPS. If you make the data repo private, you'll need to set up a deploy key and clone via SSH instead.

2. **Configure Caddy** for HTTPS:

```
# /etc/caddy/Caddyfile
builds.example.com {
    reverse_proxy localhost:8080
}
```

### iOS Notes

iOS OTA installation requires HTTPS — Caddy handles this automatically via Let's Encrypt. Devices must be registered in your ad-hoc provisioning profile to install `.ipa` builds.

### Sparkle (macOS Auto-Update)

Point your app's Sparkle `SUFeedURL` to:

```
https://builds.example.com/apps/{your-app-slug}/appcast.xml
```

The CLI generates `appcast.xml` automatically when publishing macOS builds.
Sign the final archive with Sparkle's `sign_update` tool and pass its signature
using `--sparkle-signature`. The CLI writes it into `build.json` and the matching
appcast enclosure together with the archive URL and byte count before committing
and pushing. The optional flag accepts only macOS and a 64-byte base64 Ed25519
signature; keep the private key in the signing machine's Keychain.

## Data Repo Structure

See [frontpage-data](https://github.com/hartlco/frontpage-data) for the expected folder structure and JSON schemas.

## License

MIT
