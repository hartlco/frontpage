package sparkle

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Sparkle string   `xml:"xmlns:sparkle,attr"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title string `xml:"title"`
	Link  string `xml:"link"`
	Items []Item `xml:"item"`
}

type Item struct {
	Title            string    `xml:"title"`
	Version          string    `xml:"sparkle:version"`
	ShortVersion     string    `xml:"sparkle:shortVersionString"`
	MinSystemVersion string    `xml:"sparkle:minimumSystemVersion,omitempty"`
	Description      string    `xml:"description"`
	PubDate          string    `xml:"pubDate"`
	Enclosure        Enclosure `xml:"enclosure"`
}

type Enclosure struct {
	URL       string `xml:"url,attr"`
	Length    int64  `xml:"length,attr"`
	Type      string `xml:"type,attr"`
	Signature string `xml:"sparkle:edDSASignature,attr,omitempty"`
}

type buildMeta struct {
	Version      string    `json:"version"`
	BuildNumber  string    `json:"build_number"`
	Date         time.Time `json:"date"`
	Notes        string    `json:"notes"`
	File         string    `json:"file"`
	Size         int64     `json:"size"`
	MinOSVersion string    `json:"min_os_version"`
	SparkleEdDSA string    `json:"sparkle_signature"`
	DirName      string    `json:"-"`
}

func GenerateAppcast(w io.Writer, appDir, appName, baseURL, appSlug string) error {
	buildsDir := filepath.Join(appDir, "builds")
	entries, err := os.ReadDir(buildsDir)
	if err != nil {
		return fmt.Errorf("reading builds directory: %w", err)
	}

	var builds []buildMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(buildsDir, entry.Name(), "build.json"))
		if err != nil {
			continue
		}
		var b buildMeta
		if err := json.Unmarshal(data, &b); err != nil {
			continue
		}
		b.DirName = entry.Name()
		builds = append(builds, b)
	}

	sort.Slice(builds, func(i, j int) bool {
		return builds[i].Date.After(builds[j].Date)
	})

	feedURL := baseURL + "/apps/" + appSlug + "/appcast.xml"

	rss := RSS{
		Version: "2.0",
		Sparkle: "http://www.andymatuschak.org/xml-namespaces/sparkle",
		Channel: Channel{
			Title: appName,
			Link:  feedURL,
		},
	}

	for _, b := range builds {
		downloadURL := baseURL + "/apps/" + appSlug + "/builds/" + b.DirName + "/download/" + b.File
		desc := "<p>" + b.Notes + "</p>"

		item := Item{
			Title:            "Version " + b.Version,
			Version:          b.BuildNumber,
			ShortVersion:     b.Version,
			MinSystemVersion: b.MinOSVersion,
			Description:      desc,
			PubDate:          b.Date.Format(time.RFC1123Z),
			Enclosure: Enclosure{
				URL:       downloadURL,
				Length:    b.Size,
				Type:      "application/octet-stream",
				Signature: b.SparkleEdDSA,
			},
		}
		rss.Channel.Items = append(rss.Channel.Items, item)
	}

	fmt.Fprint(w, xml.Header)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(rss)
}
