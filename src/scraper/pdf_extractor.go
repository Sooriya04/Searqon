package scraper

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"

	"src/models"
	"src/utils"
)

// ExtractPDFBytes extracts text directly from a raw PDF byte slice without extra buffering.
func ExtractPDFBytes(data []byte) (string, error) {
	if len(data) == 0 {
		return "", errors.New("empty PDF data")
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

// ExtractPDFText extracts text from a PDF document stream.
func ExtractPDFText(body io.Reader) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	return ExtractPDFBytes(data)
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
func ScrapePDFContent(rawBody []byte, targetURL string, finalURL string, startTime time.Time, opts ScrapeOptions) (models.ScrapeResult, error) {
	text, err := ExtractPDFBytes(rawBody)
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
