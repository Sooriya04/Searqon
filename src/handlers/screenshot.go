package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jung-kurt/gofpdf"

	"src/scraper"
	"src/utils"
)

type ScreenshotRequest struct {
	URL    string `json:"url"`
	Format string `json:"format"`
}

// ScreenshotHandler renders a target page as a structured PDF document.
func ScreenshotHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var req ScreenshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON request body")
		return
	}
	if req.URL == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "URL is required")
		return
	}
	if req.Format == "" {
		req.Format = "pdf"
	}

	if req.Format != "pdf" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "Only 'pdf' format is supported for rendering screenshots/documents")
		return
	}

	result, _ := scraper.ScrapeSingleURL(req.URL, "text", false)
	if result.Error != "" {
		utils.WriteError(w, http.StatusBadGateway, utils.ErrCodeScrapeFailed, result.Error)
		return
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetMargins(15, 15, 15)

	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(17, 18, 22)
	pdf.CellFormat(0, 10, result.Title, "0", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "I", 10)
	pdf.SetTextColor(100, 110, 120)
	pdf.CellFormat(0, 8, "Source: "+result.URL, "0", 1, "L", false, 0, "")
	pdf.CellFormat(0, 8, "Domain: "+result.Domain, "0", 1, "L", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Arial", "", 11)
	pdf.SetTextColor(30, 30, 30)
	
	paragraphs := splitIntoParagraphs(result.Content)
	for _, p := range paragraphs {
		pdf.MultiCell(0, 6, p, "", "L", false)
		pdf.Ln(4)
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"searqon-%s.pdf\"", result.Domain))
	err := pdf.Output(w)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, utils.ErrCodeInternalError, "Failed to render PDF: "+err.Error())
	}
}

func splitIntoParagraphs(text string) []string {
	var paras []string
	raw := strings.Split(text, "\n")
	for _, line := range raw {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			paras = append(paras, trimmed)
		}
	}
	if len(paras) == 0 && text != "" {
		paras = append(paras, text)
	}
	return paras
}
