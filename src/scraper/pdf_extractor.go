package scraper

import (
	"bytes"
	"io"

	"github.com/ledongthuc/pdf"
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
