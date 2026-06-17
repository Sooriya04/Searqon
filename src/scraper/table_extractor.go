package scraper

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ConvertTablesToMarkdown converts HTML tables to Github-Flavored Markdown.
func ConvertTablesToMarkdown(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil || doc == nil {
		return htmlContent
	}

	doc.Find("table").Each(func(i int, tableSel *goquery.Selection) {
		var mdRows []string
		var maxCols int

		var headers []string
		tableSel.Find("tr").First().Find("th, td").Each(func(j int, cell *goquery.Selection) {
			text := strings.TrimSpace(cell.Text())
			text = strings.ReplaceAll(text, "|", "\\|")
			text = strings.ReplaceAll(text, "\n", " ")
			headers = append(headers, text)
		})

		if len(headers) > 0 {
			maxCols = len(headers)
			mdRows = append(mdRows, "| "+strings.Join(headers, " | ")+" |")
			
			var separators []string
			for k := 0; k < maxCols; k++ {
				separators = append(separators, "---")
			}
			mdRows = append(mdRows, "| "+strings.Join(separators, " | ")+" |")
		}

		tableSel.Find("tr").Each(func(rIdx int, row *goquery.Selection) {
			if rIdx == 0 && len(headers) > 0 {
				return
			}

			var cells []string
			row.Find("td, th").Each(func(cIdx int, cell *goquery.Selection) {
				text := strings.TrimSpace(cell.Text())
				text = strings.ReplaceAll(text, "|", "\\|")
				text = strings.ReplaceAll(text, "\n", " ")
				cells = append(cells, text)
			})

			if len(cells) > 0 {
				if len(cells) > maxCols {
					maxCols = len(cells)
				}
				for len(cells) < maxCols {
					cells = append(cells, "")
				}
				mdRows = append(mdRows, "| "+strings.Join(cells, " | ")+" |")
			}
		})

		if len(mdRows) > 0 {
			mdTable := "\n\n" + strings.Join(mdRows, "\n") + "\n\n"
			tableSel.ReplaceWithHtml(mdTable)
		} else {
			tableSel.Remove()
		}
	})

	html, err := doc.Html()
	if err != nil || html == "" {
		return htmlContent
	}
	return html
}
