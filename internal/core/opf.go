package core

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const opfTemplateStr = `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="uuid_id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>{{.Title}}</dc:title>
    <dc:creator opf:role="aut">{{.Author}}</dc:creator>{{if .Narrator}}
    <dc:creator opf:role="nrt">{{.Narrator}}</dc:creator>{{end}}
    <dc:description>{{.Description}}</dc:description>{{if .Publisher}}
    <dc:publisher>{{.Publisher}}</dc:publisher>{{end}}{{if .Language}}
    <dc:language>{{.Language}}</dc:language>{{else}}
    <dc:language>ru</dc:language>{{end}}{{if .PublishedYear}}
    <dc:date>{{.PublishedYear}}</dc:date>{{end}}{{range .Genres}}
    <dc:subject>{{.}}</dc:subject>{{end}}{{if .Series}}
    <meta name="calibre:series" content="{{.Series}}" />{{end}}{{if .SeriesNumber}}
    <meta name="calibre:series_index" content="{{.SeriesNumber}}" />{{end}}
    <meta name="cover" content="cover.jpg" />
  </metadata>
  <manifest>
    <item id="cover" href="cover.jpg" media-type="image/jpeg" />
  </manifest>
</package>`

var opfTemplate = template.Must(template.New("opf").Parse(opfTemplateStr))

func defaultExecuteTemplate(wr io.Writer, data any) error {
	if err := opfTemplate.Execute(wr, data); err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	return nil
}

var executeTemplate = defaultExecuteTemplate

func escapeSlice(items []string) []string {
	escaped := make([]string, 0, len(items))
	for _, item := range items {
		escaped = append(escaped, html.EscapeString(item))
	}
	return escaped
}

// GenerateOPF creates an XML OPF metadata string for the given BookInfo.
func GenerateOPF(info *BookInfo) (string, error) {
	safeInfo := BookInfo{
		Title:         html.EscapeString(info.Title),
		Author:        html.EscapeString(info.Author),
		Narrator:      html.EscapeString(info.Narrator),
		Description:   html.EscapeString(info.FormattedDescription()),
		PublishedYear: html.EscapeString(info.PublishedYear),
		Publisher:     html.EscapeString(info.Publisher),
		Series:        html.EscapeString(info.Series),
		SeriesNumber:  html.EscapeString(info.SeriesNumber),
		Language:      html.EscapeString(info.Language),
		Genres:        escapeSlice(info.Genres),
	}

	var buf bytes.Buffer
	if err := executeTemplate(&buf, safeInfo); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

type creatorNode struct {
	Role  string `xml:"http://www.idpf.org/2007/opf role,attr"`
	Value string `xml:",chardata"`
}

type metaNode struct {
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Property string `xml:"property,attr"`
	Value    string `xml:",chardata"`
}

func parseStrings(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseMetas(metas []metaNode, info *BookInfo) {
	for _, m := range metas {
		switch {
		case m.Name == "calibre:series" || m.Name == "series":
			info.Series = m.Content
		case m.Name == "calibre:series_index" || m.Name == "series_index":
			info.SeriesNumber = m.Content
		case m.Property == "belongs-to-collection":
			info.Series = strings.TrimSpace(m.Value)
		case m.Property == "group-position":
			info.SeriesNumber = strings.TrimSpace(m.Value)
		}
	}
}

// ParseOPF reads a metadata.opf file and constructs a BookInfo.
func ParseOPF(path string) (*BookInfo, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	type Metadata struct {
		Title       string        `xml:"title"`
		Description string        `xml:"description"`
		Date        string        `xml:"date"`
		Publisher   string        `xml:"publisher"`
		Language    string        `xml:"language"`
		Creators    []creatorNode `xml:"creator"`
		Subjects    []string      `xml:"subject"`
		Metas       []metaNode    `xml:"meta"`
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
		Publisher:     pkg.Metadata.Publisher,
		Language:      pkg.Metadata.Language,
		Genres:        parseStrings(pkg.Metadata.Subjects),
	}

	parseMetas(pkg.Metadata.Metas, info)

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
