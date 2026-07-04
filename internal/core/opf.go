package core

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"text/template"
)

const opfTemplateStr = `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="uuid_id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>{{.Title}}</dc:title>
    <dc:creator opf:role="aut">{{.Author}}</dc:creator>{{if .Narrator}}
    <dc:creator opf:role="nrt">{{.Narrator}}</dc:creator>{{end}}
    <dc:description>{{.Description}}</dc:description>
    <dc:language>ru</dc:language>{{if .PublishedYear}}
    <dc:date>{{.PublishedYear}}</dc:date>{{end}}{{range .Genres}}
    <dc:subject>{{.}}</dc:subject>{{end}}
    <meta name="cover" content="cover.jpg" />
  </metadata>
  <manifest>
    <item id="cover" href="cover.jpg" media-type="image/jpeg" />
  </manifest>
</package>`

var opfTemplate = template.Must(template.New("opf").Parse(opfTemplateStr))

// GenerateOPF creates an XML OPF metadata string for the given BookInfo.
func GenerateOPF(info *BookInfo) (string, error) {
	safeGenres := make([]string, 0, len(info.Genres))
	for _, g := range info.Genres {
		safeGenres = append(safeGenres, html.EscapeString(g))
	}

	safeInfo := BookInfo{
		Title:         html.EscapeString(info.Title),
		Author:        html.EscapeString(info.Author),
		Narrator:      html.EscapeString(info.Narrator),
		Description:   html.EscapeString(info.Description),
		PublishedYear: html.EscapeString(info.PublishedYear),
		Genres:        safeGenres,
	}

	var buf bytes.Buffer
	if err := opfTemplate.Execute(&buf, safeInfo); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// ParseOPF reads a metadata.opf file and constructs a BookInfo.
func ParseOPF(path string) (*BookInfo, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	type Creator struct {
		Role  string `xml:"http://www.idpf.org/2007/opf role,attr"`
		Value string `xml:",chardata"`
	}

	type Metadata struct {
		Title       string    `xml:"title"`
		Description string    `xml:"description"`
		Date        string    `xml:"date"`
		Publisher   string    `xml:"publisher"`
		Creators    []Creator `xml:"creator"`
	}

	type Package struct {
		XMLName  xml.Name `xml:"package"`
		Metadata Metadata `xml:"metadata"`
	}

	var pkg Package
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("unmarshal opf: %w", err)
	}

	info := &BookInfo{
		Title:         pkg.Metadata.Title,
		Description:   pkg.Metadata.Description,
		PublishedYear: pkg.Metadata.Date,
	}

	for _, c := range pkg.Metadata.Creators {
		switch c.Role {
		case "aut":
			info.Author = c.Value
		case "nrt":
			info.Narrator = c.Value
		}
	}

	if info.Author == "" && len(pkg.Metadata.Creators) > 0 {
		info.Author = pkg.Metadata.Creators[0].Value
	}

	return info, nil
}
