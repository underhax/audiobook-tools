package scrapers

import (
	"errors"
	"strings"
	"testing"
)

type failReader struct{}

func (f failReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestAppendChild(t *testing.T) {
	parent := &node{nodeType: elementNode, data: "div"}
	c1 := &node{nodeType: elementNode, data: "p"}
	c2 := &node{nodeType: textNode, data: "text"}

	parent.appendChild(c1)
	if parent.firstChild != c1 || parent.lastChild != c1 || c1.parent != parent {
		t.Fatal("first child append failed")
	}

	parent.appendChild(c2)
	if parent.firstChild != c1 || parent.lastChild != c2 || c2.parent != parent {
		t.Fatal("second child append failed")
	}
	if c1.nextSibling != c2 || c2.prevSibling != c1 {
		t.Fatal("sibling linkage failed")
	}
}

func TestParseHTML_DoctypeAndComments(t *testing.T) {
	input := "<!DOCTYPE html><!-- a comment --><html><body><!-- incomplete"
	doc, err := parseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if doc.firstChild == nil || doc.firstChild.data != "html" {
		t.Fatalf("expected html root, got %+v", doc.firstChild)
	}
}

func TestParseHTML_XMLDeclaration(t *testing.T) {
	input := "<?xml version=\"1.0\"?></div><p>test</p>"
	doc, err := parseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if doc.firstChild == nil || doc.firstChild.data != "p" {
		t.Fatalf("expected p element, got %+v", doc.firstChild)
	}
}

func TestParseHTML_UnclosedTag(t *testing.T) {
	input := "<section>hello <"
	doc, err := parseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if doc.firstChild == nil || doc.firstChild.data != "section" {
		t.Fatalf("expected section, got %+v", doc.firstChild)
	}
}

func TestParseHTML_StyleAndScript(t *testing.T) {
	input := "<style>body > div { color: red; }</style><script>if (a < b) { run(); }</script>"
	doc, err := parseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	styleNode := doc.firstChild
	if styleNode == nil || styleNode.data != "style" {
		t.Fatalf("expected style node, got %+v", styleNode)
	}
	if styleNode.firstChild == nil || !strings.Contains(styleNode.firstChild.data, "body > div") {
		t.Fatalf("unexpected style content: %+v", styleNode.firstChild)
	}

	scriptNode := styleNode.nextSibling
	if scriptNode == nil || scriptNode.data != "script" {
		t.Fatalf("expected script node, got %+v", scriptNode)
	}
	if scriptNode.firstChild == nil || !strings.Contains(scriptNode.firstChild.data, "if (a < b)") {
		t.Fatalf("unexpected script content: %+v", scriptNode.firstChild)
	}
}

func TestParseHTML_AutoCloseLi(t *testing.T) {
	input := "<ul><li>First<li>Second</li></ul>"
	doc, err := parseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	ul := doc.firstChild
	if ul == nil || ul.data != "ul" {
		t.Fatalf("expected ul node, got %+v", ul)
	}
	li1 := ul.firstChild
	if li1 == nil || li1.data != "li" {
		t.Fatalf("expected li1, got %+v", li1)
	}
	li2 := li1.nextSibling
	if li2 == nil || li2.data != "li" {
		t.Fatalf("expected li2, got %+v", li2)
	}
}

func TestParseHTML_AttributesVariants(t *testing.T) {
	input := "<div id=\"main\" class='box' data-flag novalue= disabled/>"
	doc, err := parseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	div := doc.firstChild
	if div == nil || div.data != "div" {
		t.Fatalf("expected div, got %+v", div)
	}
	attrs := make(map[string]string)
	for _, a := range div.attr {
		attrs[a.key] = a.val
	}
	if attrs["id"] != "main" || attrs["class"] != "box" || attrs["novalue"] != "" {
		t.Fatalf("unexpected attributes: %+v", attrs)
	}
}

func TestParseHTML_VoidElementsNesting(t *testing.T) {
	input := "<p><img src=\"test.png\"><br><input type=\"text\">text</p>"
	doc, err := parseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	p := doc.firstChild
	if p == nil || p.data != "p" {
		t.Fatalf("expected p, got %+v", p)
	}
	var tags []string
	for c := p.firstChild; c != nil; c = c.nextSibling {
		if c.nodeType == elementNode {
			tags = append(tags, c.data)
		}
	}
	if len(tags) != 3 || tags[0] != tagImg || tags[1] != "br" || tags[2] != "input" {
		t.Fatalf("unexpected void tags: %v", tags)
	}
}

func TestParseHTML_IncompleteTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "incomplete doctype without gt",
			input: "<!DOCTYPE html",
		},
		{
			name:  "incomplete closing tag without gt",
			input: "<div></div",
		},
		{
			name:  "incomplete opening tag without gt",
			input: "<div><p",
		},
		{
			name:  "empty tag name with spaces",
			input: "< ><div>valid</div>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := parseHTML(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if doc == nil {
				t.Fatal("expected non-nil doc")
			}
		})
	}
}

func TestParseHTML_SpecialTagEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty script block",
			input: "<script></script>",
		},
		{
			name:  "unclosed script tag",
			input: "<script>var x = 1;",
		},
		{
			name:  "script close tag without gt",
			input: "<script>var x = 1;</script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := parseHTML(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if doc == nil {
				t.Fatal("expected non-nil doc")
			}
		})
	}
}

func TestParseHTML_ReadError(t *testing.T) {
	_, err := parseHTML(failReader{})
	if err == nil {
		t.Fatal("expected error from failReader, got nil")
	}
}

func TestParseAttributes_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "spaces only",
			input:    "   \t\n  ",
			expected: 0,
		},
		{
			name:     "slash only",
			input:    "/",
			expected: 0,
		},
		{
			name:     "unquoted value",
			input:    "width=100 height=200",
			expected: 2,
		},
		{
			name:     "empty key with equals",
			input:    " =val foo=bar",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := parseAttributes(tt.input)
			if len(res) != tt.expected {
				t.Fatalf("expected %d attrs, got %d", tt.expected, len(res))
			}
		})
	}
}
