package scrapers

import (
	"fmt"
	"html"
	"io"
	"strings"
)

type nodeType int

const (
	errorNode nodeType = iota
	textNode
	documentNode
	elementNode
	commentNode
	doctypeNode
)

type attribute struct {
	key string
	val string
}

type node struct {
	parent      *node
	firstChild  *node
	lastChild   *node
	prevSibling *node
	nextSibling *node
	data        string
	attr        []attribute
	nodeType    nodeType
}

func (n *node) appendChild(c *node) {
	c.parent = n
	if n.firstChild == nil {
		n.firstChild = c
		n.lastChild = c
		return
	}
	n.lastChild.nextSibling = c
	c.prevSibling = n.lastChild
	n.lastChild = c
}

var voidElements = map[string]bool{
	"area":   true,
	"base":   true,
	"br":     true,
	"col":    true,
	"embed":  true,
	"hr":     true,
	tagImg:   true,
	"input":  true,
	"link":   true,
	"meta":   true,
	"param":  true,
	"source": true,
	"track":  true,
	"wbr":    true,
}

func parseHTML(r io.Reader) (*node, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read html: %w", err)
	}
	s := string(b)
	doc := &node{nodeType: documentNode}
	curr := doc

	i := 0
	n := len(s)

	for i < n {
		lt := strings.IndexByte(s[i:], '<')
		if lt == -1 {
			handleText(curr, s[i:])
			break
		}
		if lt > 0 {
			handleText(curr, s[i:i+lt])
			i += lt
		}

		if strings.HasPrefix(s[i:], "<!--") || strings.HasPrefix(s[i:], "<!") || strings.HasPrefix(s[i:], "<?") {
			i = parseTagDeclaration(s, i)
			continue
		}

		if strings.HasPrefix(s[i:], "</") {
			gt := strings.IndexByte(s[i:], '>')
			if gt == -1 {
				break
			}
			tag := strings.ToLower(strings.TrimSpace(s[i+2 : i+gt]))
			curr = closeMatchingTag(curr, doc, tag)
			i += gt + 1
			continue
		}

		gt := strings.IndexByte(s[i:], '>')
		if gt == -1 {
			break
		}

		tagContent := s[i+1 : i+gt]
		i += gt + 1
		curr, i = handleOpeningTag(curr, s, tagContent, i)
	}

	return doc, nil
}

func handleText(curr *node, text string) {
	txt := strings.TrimSpace(html.UnescapeString(text))
	if txt != "" {
		curr.appendChild(&node{nodeType: textNode, data: html.UnescapeString(text)})
	}
}

func parseTagDeclaration(s string, i int) int {
	if strings.HasPrefix(s[i:], "<!--") {
		end := strings.Index(s[i:], "-->")
		if end == -1 {
			return len(s)
		}
		return i + end + 3
	}

	gt := strings.IndexByte(s[i:], '>')
	if gt == -1 {
		return len(s)
	}
	return i + gt + 1
}

func closeMatchingTag(curr, doc *node, tag string) *node {
	for p := curr; p != nil && p != doc; p = p.parent {
		if p.data == tag {
			return p.parent
		}
	}
	return curr
}

func handleOpeningTag(curr *node, s, tagContent string, i int) (next *node, nextI int) {
	selfClosing := strings.HasSuffix(tagContent, "/")
	if selfClosing {
		tagContent = strings.TrimSpace(strings.TrimSuffix(tagContent, "/"))
	}

	parts := strings.SplitN(tagContent, " ", 2)
	tagName := strings.ToLower(strings.TrimSpace(parts[0]))
	if tagName == "" {
		return curr, i
	}

	var attrs []attribute
	if len(parts) > 1 {
		attrs = parseAttributes(parts[1])
	}

	if tagName == "li" && curr != nil && curr.data == "li" {
		curr = curr.parent
	}

	elem := &node{
		nodeType: elementNode,
		data:     tagName,
		attr:     attrs,
	}
	curr.appendChild(elem)

	if selfClosing || voidElements[tagName] {
		return curr, i
	}

	if tagName == "script" || tagName == "style" {
		return curr, handleSpecialTag(s, i, tagName, elem)
	}

	return elem, i
}

func handleSpecialTag(s string, i int, tagName string, elem *node) int {
	closeTag := "</" + tagName
	end := strings.Index(strings.ToLower(s[i:]), closeTag)
	if end == -1 {
		return i
	}

	raw := s[i : i+end]
	if raw != "" {
		elem.appendChild(&node{nodeType: textNode, data: raw})
	}

	closeGt := strings.IndexByte(s[i+end:], '>')
	if closeGt != -1 {
		return i + end + closeGt + 1
	}
	return i + end + len(closeTag)
}

func parseAttributes(s string) []attribute {
	var attrs []attribute
	i := 0
	n := len(s)

	for i < n {
		i = skipWhitespace(s, i, n)
		if i >= n || s[i] == '/' || s[i] == '>' {
			break
		}

		keyStart := i
		for i < n && s[i] != '=' && !isAttrDelimiter(s[i]) {
			i++
		}
		key := strings.ToLower(s[keyStart:i])
		if key == "" {
			for i < n && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' && s[i] != '>' {
				i++
			}
			continue
		}

		var rawVal string
		rawVal, i = parseAttrValue(s, i, n)
		attrs = append(attrs, attribute{
			key: key,
			val: html.UnescapeString(rawVal),
		})
	}

	return attrs
}

func parseAttrValue(s string, i, n int) (val string, nextI int) {
	if i >= n || s[i] != '=' {
		return "", i
	}
	i++
	if i >= n || isAttrDelimiter(s[i]) {
		return "", i
	}

	quote := s[i]
	if quote == '"' || quote == '\'' {
		i++
		valStart := i
		for i < n && s[i] != quote {
			i++
		}
		val = s[valStart:i]
		if i < n {
			i++
		}
		return val, i
	}

	valStart := i
	for i < n && !isAttrDelimiter(s[i]) {
		i++
	}
	return s[valStart:i], i
}

func isAttrDelimiter(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '/' || c == '>'
}

func skipWhitespace(s string, i, n int) int {
	for i < n && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	return i
}
