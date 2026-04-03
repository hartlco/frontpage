package plist

import (
	"fmt"
	"io"
)

type ManifestParams struct {
	DownloadURL    string
	BundleID       string
	BundleVersion  string
	Title          string
}

func WriteManifest(w io.Writer, p ManifestParams) error {
	_, err := fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>items</key>
	<array>
		<dict>
			<key>assets</key>
			<array>
				<dict>
					<key>kind</key>
					<string>software-package</string>
					<key>url</key>
					<string>%s</string>
				</dict>
			</array>
			<key>metadata</key>
			<dict>
				<key>bundle-identifier</key>
				<string>%s</string>
				<key>bundle-version</key>
				<string>%s</string>
				<key>kind</key>
				<string>software</string>
				<key>title</key>
				<string>%s</string>
			</dict>
		</dict>
	</array>
</dict>
</plist>`, p.DownloadURL, p.BundleID, p.BundleVersion, p.Title)
	return err
}
