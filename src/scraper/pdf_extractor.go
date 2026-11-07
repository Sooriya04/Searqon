package scraper

import (
	"bytes"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"

	"src/models"
	"src/utils"
)

// ExtractPDFText extracts text from a PDF document stream.
func ExtractPDFText(body io.Reader) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}

	readerAt := bytes.NewReader(data)
	r, err := pdf.NewReader(readerAt, int64(len(data)))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err == nil {
		_, err = io.Copy(&buf, b)
		if err == nil {
			return buf.String(), nil
		}
	}

	buf.Reset()
	numPages := r.NumPage()
	for i := 1; i <= numPages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err == nil {
			buf.WriteString(text)
			buf.WriteString("\n")
		}
	}

	return buf.String(), nil
}

// IsPDFResponse determines if the request target or response indicates a PDF document.
func IsPDFResponse(targetURL string, contentType string) bool {
	if strings.Contains(strings.ToLower(contentType), "application/pdf") {
		return true
	}
	if parsed, err := url.Parse(targetURL); err == nil {
		path := strings.ToLower(parsed.Path)
		if strings.HasSuffix(path, ".pdf") {
			return true
		}
	} else if strings.HasSuffix(strings.ToLower(targetURL), ".pdf") {
		return true
	}
	return false
}

// ScrapePDFContent extracts text and builds a models.ScrapeResult from a PDF stream or byte slice.
func ScrapePDFContent(rawBody string, targetURL string, finalURL string, startTime time.Time, opts ScrapeOptions) (models.ScrapeResult, error) {
	text, err := ExtractPDFText(strings.NewReader(rawBody))
	if err != nil {
		return models.ScrapeResult{}, err
	}

	text = utils.CleanText(text)
	parsedURL, _ := url.Parse(targetURL)
	title := ""
	if parsedURL != nil && parsedURL.Path != "" {
		title = parsedURL.Path[strings.LastIndex(parsedURL.Path, "/")+1:]
	}
	if title == "" {
		title = "PDF Document"
	}

	res := models.ScrapeResult{
		URL:              targetURL,
		CanonicalURL:     finalURL,
		Title:            title,
		Content:          text,
		Markdown:         text,
		WordCount:        utils.CountWords(text),
		StatusCode:       200,
		ContentType:      "application/pdf",
		RenderMethod:     "go",
		EscalationTier:   string(TierFastHTTP),
		BotDetected:      false,
		ExtractionMethod: "pdf",
	}
	if parsedURL != nil {
		res.Domain = parsedURL.Hostname()
	}

	finalizeScrapeResult(&res, startTime, opts, "pdf")
	return res, nil
}
