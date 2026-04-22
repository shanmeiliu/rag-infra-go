package sources

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

func ExtractText(filename string, content []byte) (string, string, error) {
	lower := strings.ToLower(filename)

	switch {
	case strings.HasSuffix(lower, ".txt"), strings.HasSuffix(lower, ".md"):
		return normalizeExtractedText(string(content)), "plain_text", nil

	case strings.HasSuffix(lower, ".docx"):
		text, err := extractDOCXText(content)
		if err != nil {
			return "", "", err
		}
		return normalizeExtractedText(text), "docx_xml", nil

	default:
		// Unsupported file types return empty (no ingestion)
		return "", "", nil
	}
}

func extractDOCXText(content []byte) (string, error) {
	readerAt := bytes.NewReader(content)

	zr, err := zip.NewReader(readerAt, int64(len(content)))
	if err != nil {
		return "", err
	}

	var documentXML []byte

	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}

			documentXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			break
		}
	}

	if len(documentXML) == 0 {
		return "", fmt.Errorf("document.xml not found in docx")
	}

	return parseWordXML(documentXML)
}

type wordDocument struct {
	Body wordBody `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main body"`
}

type wordBody struct {
	Paragraphs []wordParagraph `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main p"`
}

type wordParagraph struct {
	Runs []wordRun `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main r"`
}

type wordRun struct {
	Texts []string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main t"`
}

func parseWordXML(data []byte) (string, error) {
	var doc wordDocument

	if err := xml.Unmarshal(data, &doc); err != nil {
		return "", err
	}

	var paragraphs []string

	for _, p := range doc.Body.Paragraphs {
		var sb strings.Builder

		for _, r := range p.Runs {
			for _, t := range r.Texts {
				sb.WriteString(t)
			}
		}

		text := strings.TrimSpace(sb.String())
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
	}

	return strings.Join(paragraphs, "\n\n"), nil
}

func normalizeExtractedText(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			if len(out) == 0 || out[len(out)-1] == "" {
				continue
			}
			out = append(out, "")
			continue
		}

		out = append(out, line)
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}
