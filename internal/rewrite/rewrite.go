package rewrite

import (
	"context"
	"strings"

	"github.com/shanmeiliu/rag-infra-go/internal/memory"
)

type SimpleRewriter struct{}

func NewSimpleRewriter() *SimpleRewriter {
	return &SimpleRewriter{}
}

func (r *SimpleRewriter) Rewrite(ctx context.Context, query string, history []memory.Message) (string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return q, nil
	}

	expanded := q

	replacements := map[string]string{
		" she ":      " Charmaine ",
		" her ":      " Charmaine ",
		" she's ":    " Charmaine is ",
		" is she ":   " is Charmaine ",
		" does she ": " does Charmaine ",
		" can she ":  " can Charmaine ",
		" has she ":  " has Charmaine ",
		" did she ":  " did Charmaine ",
	}

	padded := " " + expanded + " "
	lower := strings.ToLower(padded)

	for from, to := range replacements {
		if strings.Contains(lower, from) {
			padded = replaceCaseInsensitive(padded, from, to)
			lower = strings.ToLower(padded)
		}
	}

	expanded = strings.TrimSpace(padded)

	if containsAny(lower, []string{"work in canada", "allowed to work", "authorized", "authorised", "lawfully", "work authorization", "citizen"}) {
		expanded += " work authorization Canada authorized to work lawfully Canadian citizen eligible work permit no sponsorship"
	}

	if containsAny(lower, []string{"ai system", "ai systems", "llm", "rag", "genai", "machine learning"}) {
		expanded += " AI LLM GenAI RAG retrieval augmented generation embeddings vector database pgvector LangChain Ollama OpenAI"
	}

	if containsAny(lower, []string{"project", "projects", "built", "app", "application"}) {
		expanded += " projects applications built backend frontend RAG language app blog app GitHub repository"
	}

	return expanded, nil
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func replaceCaseInsensitive(input, old, new string) string {
	lowerInput := strings.ToLower(input)
	lowerOld := strings.ToLower(old)

	var out strings.Builder
	start := 0

	for {
		idx := strings.Index(lowerInput[start:], lowerOld)
		if idx == -1 {
			out.WriteString(input[start:])
			break
		}

		idx += start
		out.WriteString(input[start:idx])
		out.WriteString(new)
		start = idx + len(old)
	}

	return out.String()
}
