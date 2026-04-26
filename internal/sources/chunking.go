package sources

import (
	"regexp"
	"strings"
)

var qStartRe = regexp.MustCompile(`(?m)^\s*Q\s*:`)

func ChunkText(text string, maxChars int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	if chunks := chunkQAText(text, maxChars); len(chunks) > 0 {
		return chunks
	}

	return chunkGenericText(text, maxChars)
}

func chunkQAText(text string, maxChars int) []string {
	if maxChars <= 0 {
		maxChars = 1800
	}

	indexes := qStartRe.FindAllStringIndex(text, -1)
	if len(indexes) < 2 {
		return nil
	}

	var chunks []string

	for i, idx := range indexes {
		start := idx[0]
		end := len(text)
		if i+1 < len(indexes) {
			end = indexes[i+1][0]
		}

		part := strings.TrimSpace(text[start:end])
		part = strings.Trim(part, "-_ \n\t")

		if part == "" {
			continue
		}

		if !strings.Contains(strings.ToLower(part), "a:") {
			continue
		}

		if len(part) <= maxChars {
			chunks = append(chunks, part)
			continue
		}

		chunks = append(chunks, chunkGenericText(part, maxChars)...)
	}

	return chunks
}

func chunkGenericText(text string, maxChars int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	if maxChars <= 0 {
		maxChars = 1800
	}

	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var current strings.Builder

	flush := func() {
		c := strings.TrimSpace(current.String())
		if c != "" {
			chunks = append(chunks, c)
		}
		current.Reset()
	}

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if current.Len() == 0 {
			current.WriteString(p)
			continue
		}

		if current.Len()+2+len(p) <= maxChars {
			current.WriteString("\n\n")
			current.WriteString(p)
			continue
		}

		flush()

		if len(p) <= maxChars {
			current.WriteString(p)
			continue
		}

		parts := splitLongParagraph(p, maxChars)
		for i, part := range parts {
			if i == len(parts)-1 {
				current.WriteString(part)
			} else {
				chunks = append(chunks, part)
			}
		}
	}

	flush()
	return chunks
}

func splitLongParagraph(p string, maxChars int) []string {
	words := strings.Fields(p)
	if len(words) == 0 {
		return nil
	}

	var parts []string
	var current strings.Builder

	flush := func() {
		c := strings.TrimSpace(current.String())
		if c != "" {
			parts = append(parts, c)
		}
		current.Reset()
	}

	for _, w := range words {
		if current.Len() == 0 {
			current.WriteString(w)
			continue
		}

		if current.Len()+1+len(w) <= maxChars {
			current.WriteString(" ")
			current.WriteString(w)
			continue
		}

		flush()
		current.WriteString(w)
	}

	flush()
	return parts
}
