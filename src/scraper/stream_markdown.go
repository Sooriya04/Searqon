package scraper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// ErrLimitReached is returned internally by LimitedWriter when maxBytes is exceeded.
var ErrLimitReached = errors.New("markdown stream byte limit reached")

const (
	defaultTruncationMarker = "\n\n[truncated]\n"
	maxListDepth            = 32
)

// LimitedWriter caps output bytes and terminates early once limit is hit.
type LimitedWriter struct {
	inner     io.Writer
	remaining int
	truncated bool
	hasLimit  bool
}

// NewLimitedWriter creates a writer with an optional byte ceiling.
func NewLimitedWriter(w io.Writer, maxBytes int) *LimitedWriter {
	lw := &LimitedWriter{
		inner:    w,
		hasLimit: maxBytes > 0,
	}
	if maxBytes > 0 {
		lw.remaining = maxBytes
	}
	return lw
}

// Write consumes bytes up to remaining capacity.
func (lw *LimitedWriter) Write(p []byte) (int, error) {
	if !lw.hasLimit {
		return lw.inner.Write(p)
	}
	if lw.remaining <= 0 {
		lw.truncated = true
		return 0, ErrLimitReached
	}
	if len(p) <= lw.remaining {
		n, err := lw.inner.Write(p)
		lw.remaining -= n
		return n, err
	}

	// Boundary hit: write valid UTF-8 prefix within remaining bytes
	safeLen := truncateUTF8Safe(p, lw.remaining)
	if safeLen > 0 {
		_, _ = lw.inner.Write(p[:safeLen])
	}
	lw.remaining = 0
	lw.truncated = true
	_, _ = lw.inner.Write([]byte(defaultTruncationMarker))
	return safeLen, ErrLimitReached
}

func truncateUTF8Safe(b []byte, maxLen int) int {
	if maxLen >= len(b) {
		return len(b)
	}
	if maxLen <= 0 {
		return 0
	}
	// Step backwards until we hit a valid UTF-8 rune boundary
	for maxLen > 0 && !utf8.RuneStart(b[maxLen]) {
		maxLen--
	}
	return maxLen
}

type listState struct {
	ordered bool
	counter int
}

// MarkdownStateMachine streams HTML tokens into clean GitHub-Flavored Markdown.
type MarkdownStateMachine struct {
	w               *LimitedWriter
	skipDepth       int
	inPre           bool
	preFenceWritten bool
	inCode          bool
	inBlockquote    bool

	// Links
	inLink   bool
	linkHref string
	linkText strings.Builder

	// Lists
	listStack [maxListDepth]listState
	listDepth int

	// Tables
	inTable         bool
	inCell          bool
	isHeader        bool
	tableHeaderDone bool
	currentCell     strings.Builder
	currentRow      []string
	headerColCount  int

	// Whitespace normalization
	trailingNewlines int
	lastCharSpace    bool
}

// NewMarkdownStateMachine initializes state for streaming conversion.
func NewMarkdownStateMachine(w *LimitedWriter) *MarkdownStateMachine {
	return &MarkdownStateMachine{
		w:                w,
		trailingNewlines: 2, // avoid leading newlines at start of document
		lastCharSpace:    true,
	}
}

// ConvertHTMLToMarkdownStream converts HTML reader to Markdown bounded by maxBytes.
// If maxBytes <= 0, no truncation limit is enforced.
func ConvertHTMLToMarkdownStream(r io.Reader, maxBytes int) (string, bool, error) {
	var buf bytes.Buffer
	lw := NewLimitedWriter(&buf, maxBytes)
	sm := NewMarkdownStateMachine(lw)

	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			err := z.Err()
			if errors.Is(err, io.EOF) {
				break
			}
			return buf.String(), lw.truncated, err
		}

		err := sm.processToken(z, tt)
		if err != nil {
			if errors.Is(err, ErrLimitReached) {
				return buf.String(), true, nil
			}
			return buf.String(), lw.truncated, err
		}
	}

	return strings.TrimSpace(buf.String()), lw.truncated, nil
}

func (sm *MarkdownStateMachine) processToken(z *html.Tokenizer, tt html.TokenType) error {
	switch tt {
	case html.StartTagToken, html.SelfClosingTagToken:
		tagNameBytes, hasAttr := z.TagName()
		tagName := string(tagNameBytes)
		return sm.handleStartTag(tagName, hasAttr, z, tt == html.SelfClosingTagToken)

	case html.EndTagToken:
		tagNameBytes, _ := z.TagName()
		tagName := string(tagNameBytes)
		return sm.handleEndTag(tagName)

	case html.TextToken:
		if sm.skipDepth > 0 {
			return nil
		}
		raw := z.Text()
		return sm.handleText(raw)
	}
	return nil
}

func (sm *MarkdownStateMachine) handleStartTag(tag string, hasAttr bool, z *html.Tokenizer, selfClosing bool) error {
	// Skip tags
	switch tag {
	case "script", "style", "svg", "noscript", "iframe", "canvas", "video", "audio", "template":
		if !selfClosing {
			sm.skipDepth++
		}
		return nil
	}

	if sm.skipDepth > 0 {
		return nil
	}

	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(tag[1] - '0')
		if err := sm.ensureNewlines(2); err != nil {
			return err
		}
		prefix := strings.Repeat("#", level) + " "
		return sm.writeRaw(prefix)

	case "p", "div", "article", "section", "main", "header", "footer", "aside":
		return sm.ensureNewlines(2)

	case "br":
		return sm.writeRaw("\n")

	case "hr":
		if err := sm.ensureNewlines(2); err != nil {
			return err
		}
		if err := sm.writeRaw("---"); err != nil {
			return err
		}
		return sm.ensureNewlines(2)

	case "strong", "b":
		return sm.writeRaw("**")

	case "em", "i":
		return sm.writeRaw("*")

	case "del", "s", "strike":
		return sm.writeRaw("~~")

	case "code":
		if !sm.inPre {
			sm.inCode = true
			return sm.writeRaw("`")
		}
		if !sm.preFenceWritten {
			class := extractAttr(z, hasAttr, "class")
			lang := extractCodeLanguage(class)
			sm.preFenceWritten = true
			return sm.writeRaw("```" + lang + "\n")
		}

	case "pre":
		sm.inPre = true
		sm.preFenceWritten = false
		return sm.ensureNewlines(2)

	case "blockquote":
		sm.inBlockquote = true
		if err := sm.ensureNewlines(2); err != nil {
			return err
		}
		return sm.writeRaw("> ")

	case "ul":
		if err := sm.ensureNewlines(1); err != nil {
			return err
		}
		if sm.listDepth < maxListDepth {
			sm.listStack[sm.listDepth] = listState{ordered: false, counter: 0}
			sm.listDepth++
		}

	case "ol":
		if err := sm.ensureNewlines(1); err != nil {
			return err
		}
		if sm.listDepth < maxListDepth {
			sm.listStack[sm.listDepth] = listState{ordered: true, counter: 0}
			sm.listDepth++
		}

	case "li":
		if err := sm.ensureNewlines(1); err != nil {
			return err
		}
		if sm.listDepth > 0 {
			st := &sm.listStack[sm.listDepth-1]
			st.counter++
			indent := strings.Repeat("  ", sm.listDepth-1)
			var bullet string
			if st.ordered {
				bullet = fmt.Sprintf("%d. ", st.counter)
			} else {
				bullet = "* "
			}
			return sm.writeRaw(indent + bullet)
		}
		return sm.writeRaw("* ")

	case "a":
		href := extractAttr(z, hasAttr, "href")
		if href != "" && !strings.HasPrefix(href, "javascript:") {
			sm.inLink = true
			sm.linkHref = href
			sm.linkText.Reset()
		}

	case "img":
		src := extractAttr(z, hasAttr, "src")
		alt := extractAttr(z, hasAttr, "alt")
		if src != "" {
			if alt == "" {
				alt = "image"
			}
			return sm.writeRaw(fmt.Sprintf("![%s](%s)", alt, src))
		}

	case "table":
		sm.inTable = true
		sm.tableHeaderDone = false
		sm.headerColCount = 0
		sm.currentRow = nil
		return sm.ensureNewlines(2)

	case "tr":
		sm.currentRow = sm.currentRow[:0]

	case "th":
		sm.inCell = true
		sm.isHeader = true
		sm.currentCell.Reset()

	case "td":
		sm.inCell = true
		sm.isHeader = false
		sm.currentCell.Reset()
	}

	return nil
}

func (sm *MarkdownStateMachine) handleEndTag(tag string) error {
	switch tag {
	case "script", "style", "svg", "noscript", "iframe", "canvas", "video", "audio", "template":
		if sm.skipDepth > 0 {
			sm.skipDepth--
		}
		return nil
	}

	if sm.skipDepth > 0 {
		return nil
	}

	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return sm.ensureNewlines(2)

	case "p", "div", "article", "section", "main", "header", "footer", "aside":
		return sm.ensureNewlines(2)

	case "strong", "b":
		return sm.writeRaw("**")

	case "em", "i":
		return sm.writeRaw("*")

	case "del", "s", "strike":
		return sm.writeRaw("~~")

	case "code":
		if !sm.inPre && sm.inCode {
			sm.inCode = false
			return sm.writeRaw("`")
		}

	case "pre":
		sm.inPre = false
		if !sm.preFenceWritten {
			_ = sm.writeRaw("```\n")
			sm.preFenceWritten = true
		}
		if err := sm.ensureNewlines(1); err != nil {
			return err
		}
		if err := sm.writeRaw("```"); err != nil {
			return err
		}
		return sm.ensureNewlines(2)

	case "blockquote":
		sm.inBlockquote = false
		return sm.ensureNewlines(2)

	case "ul", "ol":
		if sm.listDepth > 0 {
			sm.listDepth--
		}
		return sm.ensureNewlines(1)

	case "li":
		return sm.ensureNewlines(1)

	case "a":
		if sm.inLink {
			sm.inLink = false
			txt := strings.Join(strings.Fields(sm.linkText.String()), " ")
			sm.linkText.Reset()
			if txt != "" {
				return sm.writeRaw(fmt.Sprintf("[%s](%s)", txt, sm.linkHref))
			}
		}

	case "th", "td":
		sm.inCell = false
		cellText := strings.TrimSpace(sm.currentCell.String())
		cellText = strings.ReplaceAll(cellText, "|", "\\|")
		cellText = strings.ReplaceAll(cellText, "\n", " ")
		sm.currentRow = append(sm.currentRow, cellText)
		sm.currentCell.Reset()

	case "tr":
		if len(sm.currentRow) > 0 {
			rowStr := "| " + strings.Join(sm.currentRow, " | ") + " |\n"
			if err := sm.writeRaw(rowStr); err != nil {
				return err
			}
			if !sm.tableHeaderDone {
				sm.headerColCount = len(sm.currentRow)
				dashes := make([]string, sm.headerColCount)
				for i := range dashes {
					dashes[i] = "---"
				}
				sepStr := "| " + strings.Join(dashes, " | ") + " |\n"
				if err := sm.writeRaw(sepStr); err != nil {
					return err
				}
				sm.tableHeaderDone = true
			}
			sm.currentRow = sm.currentRow[:0]
		}

	case "table":
		sm.inTable = false
		return sm.ensureNewlines(2)
	}

	return nil
}

func (sm *MarkdownStateMachine) handleText(raw []byte) error {
	if sm.inPre {
		if !sm.preFenceWritten {
			if err := sm.writeRaw("```\n"); err != nil {
				return err
			}
			sm.preFenceWritten = true
		}
		return sm.writeRaw(string(raw))
	}

	if sm.inCell {
		sm.currentCell.Write(raw)
		return nil
	}

	if sm.inLink {
		sm.linkText.Write(raw)
		return nil
	}

	// Whitespace normalization: collapse multiple spaces/newlines into a single space
	var buf strings.Builder
	for i := 0; i < len(raw); {
		r, size := utf8.DecodeRune(raw[i:])
		i += size

		if unicode.IsSpace(r) {
			if !sm.lastCharSpace {
				buf.WriteByte(' ')
				sm.lastCharSpace = true
			}
		} else {
			buf.WriteRune(r)
			sm.lastCharSpace = false
			sm.trailingNewlines = 0
		}
	}

	if buf.Len() > 0 {
		return sm.writeRaw(buf.String())
	}
	return nil
}

func (sm *MarkdownStateMachine) ensureNewlines(n int) error {
	for sm.trailingNewlines < n {
		if err := sm.writeRaw("\n"); err != nil {
			return err
		}
	}
	return nil
}

func (sm *MarkdownStateMachine) writeRaw(s string) error {
	if len(s) == 0 {
		return nil
	}
	if sm.inLink {
		sm.linkText.WriteString(s)
		return nil
	}
	if sm.inCell {
		sm.currentCell.WriteString(s)
		return nil
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			sm.trailingNewlines++
			sm.lastCharSpace = true
		} else {
			sm.trailingNewlines = 0
			if !unicode.IsSpace(rune(s[i])) {
				sm.lastCharSpace = false
			}
		}
	}
	_, err := sm.w.Write([]byte(s))
	return err
}

func extractAttr(z *html.Tokenizer, hasAttr bool, targetKey string) string {
	if !hasAttr {
		return ""
	}
	for {
		key, val, more := z.TagAttr()
		if string(key) == targetKey {
			return string(val)
		}
		if !more {
			break
		}
	}
	return ""
}

func extractCodeLanguage(class string) string {
	for _, part := range strings.Fields(class) {
		part = strings.ToLower(part)
		if strings.HasPrefix(part, "language-") {
			return strings.TrimPrefix(part, "language-")
		}
		if strings.HasPrefix(part, "lang-") {
			return strings.TrimPrefix(part, "lang-")
		}
	}
	return ""
}

