# Frontpage

Self-hosted app distribution server — like HockeyApp, but simple and yours.

Serve iOS, Android, and macOS builds to your testers from a clean web UI. No database, no accounts — just a GitHub repo with your build artifacts and JSON metadata.

## Features

- **Web UI** — Browse apps, view builds, download artifacts
- **iOS OTA Install** — Generates `manifest.plist` on the fly for `itms-services://` installs
- **Android APK** — Direct download with QR code for easy install
- **macOS + Sparkle** — Serves `appcast.xml` for automatic updates via Sparkle framework
- **QR Codes** — Auto-generated QR codes for iOS and Android builds
- **Git-based storage** — Your builds live in a separate GitHub repo (with Git LFS). The server syncs via `git pull`
- **CLI tool** — `frontpage-cli publish` structures builds, generates metadata, and pushes to the data repo
- **Docker** — Single container, easy to deploy

## Architecture

```
┌──────────────┐     git push      ┌──────────────────┐     git pull      ┌──────────────┐
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
# Clone the data repo next to docker-compose.yml
git clone git@github.com:hartlco/frontpage-data.git

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
  --base-url https://builds.hartl.co

# Publish a macOS build (also generates appcast.xml for Sparkle)
frontpage-cli publish \
  --data-dir ./frontpage-data \
  --platform macos \
  --app my-mac-app \
  --version 2.0.0 \
  --build 150 \
  --file ./build/MyApp.dmg \
  --notes "New features" \
  --base-url https://builds.hartl.co
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
| `--base-url` | Yes | Server URL (e.g. `https://builds.hartl.co`) |
| `--notes` | No | Release notes |
| `--name` | No | Display name (defaults to slug, used for new apps) |
| `--bundle-id` | No | Bundle identifier (used for new apps) |
| `--min-os` | No | Minimum OS version |

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `FRONTPAGE_DATA_DIR` | `./testdata` | Path to the data repo |
| `FRONTPAGE_BASE_URL` | `http://localhost:8080` | Public URL of the server |
| `FRONTPAGE_SYNC_INTERVAL` | `900` | Seconds between `git pull` (0 to disable) |
| `FRONTPAGE_PORT` | `8080` | HTTP listen port |

## Deployment

### Prerequisites

- Ubuntu VPS with Docker
- A domain pointing to the VPS (e.g. `builds.hartl.co`)
- [Caddy](https://caddyserver.com/) for automatic HTTPS (required for iOS OTA)

### Setup

1. **Create a deploy key** for the data repo:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/frontpage-data-deploy -N ""
# Add the public key to github.com/hartlco/frontpage-data → Settings → Deploy keys
```

2. **Configure SSH:**

```
# ~/.ssh/config
Host github-frontpage
    HostName github.com
    User git
    IdentityFile ~/.ssh/frontpage-data-deploy
```

3. **Clone and start:**

```bash
mkdir -p /srv/frontpage && cd /srv/frontpage
git clone git@github-frontpage:hartlco/frontpage-data.git
# Download docker-compose.yml, then:
docker compose up -d
```

4. **Configure Caddy** for HTTPS:

```
# /etc/caddy/Caddyfile
builds.hartl.co {
    reverse_proxy localhost:8080
}
```

### iOS Notes

iOS OTA installation requires HTTPS — Caddy handles this automatically via Let's Encrypt. Devices must be registered in your ad-hoc provisioning profile to install `.ipa` builds.

### Sparkle (macOS Auto-Update)

Point your app's Sparkle `SUFeedURL` to:

```
https://builds.hartl.co/apps/{your-app-slug}/appcast.xml
```

The CLI generates `appcast.xml` automatically when publishing macOS builds.

## Data Repo Structure

See [frontpage-data](https://github.com/hartlco/frontpage-data) for the expected folder structure and JSON schemas.

## License

MIT
