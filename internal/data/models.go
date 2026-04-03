package data

import "time"

type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformMacOS   Platform = "macos"
)

type App struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Platform    Platform `json:"platform"`
	BundleID    string   `json:"bundle_id"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Builds      []*Build `json:"-"`
	DataDir     string   `json:"-"`
}

type Build struct {
	Version       string    `json:"version"`
	BuildNumber   string    `json:"build_number"`
	Date          time.Time `json:"date"`
	Notes         string    `json:"notes"`
	File          string    `json:"file"`
	Size          int64     `json:"size"`
	MinOSVersion  string    `json:"min_os_version,omitempty"`
	SparkleEdDSA  string    `json:"sparkle_signature,omitempty"`
	VersionString string    `json:"-"`
	App           *App      `json:"-"`
}

func (b *Build) DownloadPath() string {
	return "/apps/" + b.App.Slug + "/builds/" + b.VersionString + "/download/" + b.File
}

func (b *Build) ManifestPath() string {
	return "/apps/" + b.App.Slug + "/builds/" + b.VersionString + "/manifest.plist"
}

func (b *Build) QRPath() string {
	return "/apps/" + b.App.Slug + "/builds/" + b.VersionString + "/qr.png"
}
