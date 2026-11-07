package scraper

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jung-kurt/gofpdf"
)

func generateTestPDF(title, content string) []byte {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, title)
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(0, 6, content, "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestPDFScraperBypassesEscalation(t *testing.T) {
	pdfBytes := generateTestPDF("Academic Research Paper", "This is an important whitepaper on distributed computing and cryptographic provenance. It contains substantial text for extraction and validation.")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		w.Write(pdfBytes)
	}))
	defer ts.Close()

	opts := ScrapeOptions{Format: "markdown", StealthLevel: "auto"}
	res, _, err := ExecuteSmartEscalationScrape(ts.URL+"/paper.pdf", "Searqon/2.0", opts, time.Now())
	if err != nil {
		t.Fatalf("Expected PDF scraping to succeed without escalation failure, got err: %v", err)
	}

	if !res.Scraped {
		t.Fatalf("Expected res.Scraped to be true, got error: %s", res.Error)
	}

	if res.ContentType != "application/pdf" {
		t.Errorf("Expected ContentType application/pdf, got %s", res.ContentType)
	}

	if res.ExtractionMethod != "pdf" {
		t.Errorf("Expected ExtractionMethod 'pdf', got %s", res.ExtractionMethod)
	}

	if !strings.Contains(res.Content, "distributed computing") {
		t.Errorf("Expected PDF content to contain 'distributed computing', got: %s", res.Content)
	}

	if res.WordCount == 0 {
		t.Errorf("Expected non-zero WordCount from PDF extraction, got 0")
	}
}
