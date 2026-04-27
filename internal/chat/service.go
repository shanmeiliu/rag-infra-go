package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shanmeiliu/rag-infra-go/internal/memory"
	"github.com/shanmeiliu/rag-infra-go/pkg/embedding"
	"github.com/shanmeiliu/rag-infra-go/pkg/llm"
)

type Rewriter interface {
	Rewrite(ctx context.Context, query string, history []memory.Message) (string, error)
}

type Retriever interface {
	Retrieve(ctx context.Context, query string, embedding []float32, filters map[string]any) ([]Document, error)
}

type MemoryStore interface {
	Load(ctx context.Context, sessionID string) ([]memory.Message, error)
	Save(ctx context.Context, sessionID string, msg memory.Message) error
}

type Document struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Source   string         `json:"source"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type Dependencies struct {
	Rewriter  Rewriter
	Retriever Retriever
	Memory    MemoryStore
	LLM       llm.Client
	Embedder  embedding.Client
}

type Service struct {
	rewriter  Rewriter
	retriever Retriever
	memory    MemoryStore
	llm       llm.Client
	embedder  embedding.Client
}

type Request struct {
	SessionID string         `json:"session_id"`
	Query     string         `json:"query"`
	Filters   map[string]any `json:"filters,omitempty"`
}

type Response struct {
	RewrittenQuery string         `json:"rewritten_query"`
	Documents      []Document     `json:"documents"`
	Answer         string         `json:"answer"`
	Filters        map[string]any `json:"filters,omitempty"`
	Mode           string         `json:"mode,omitempty"`
}

func NewService(dep Dependencies) *Service {
	return &Service{
		rewriter:  dep.Rewriter,
		retriever: dep.Retriever,
		memory:    dep.Memory,
		llm:       dep.LLM,
		embedder:  dep.Embedder,
	}
}

func (s *Service) Ask(ctx context.Context, req Request) (*Response, error) {
	if strings.TrimSpace(req.SessionID) == "" {
		return nil, errors.New("session_id is required")
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, errors.New("query is required")
	}

	mode, cleanQuery := extractMode(req.Query)

	history, err := s.memory.Load(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	rewritten, err := s.rewriter.Rewrite(ctx, cleanQuery, history)
	if err != nil {
		return nil, err
	}

	embeddingVec, err := s.embedder.Embed(ctx, rewritten)
	if err != nil {
		return nil, err
	}

	docs, err := s.retriever.Retrieve(ctx, rewritten, embeddingVec, req.Filters)
	if err != nil {
		return nil, err
	}

	prompt := buildPrompt(mode, rewritten, docs, history)

	answer, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	answer = appendCitationHint(answer, docs)

	_ = s.memory.Save(ctx, req.SessionID, memory.Message{
		Role:    "user",
		Content: cleanQuery,
	})
	_ = s.memory.Save(ctx, req.SessionID, memory.Message{
		Role:    "assistant",
		Content: answer,
	})

	return &Response{
		RewrittenQuery: rewritten,
		Documents:      docs,
		Answer:         answer,
		Filters:        req.Filters,
		Mode:           mode,
	}, nil
}

func (s *Service) Stream(ctx context.Context, req Request) (<-chan string, error) {
	if strings.TrimSpace(req.SessionID) == "" {
		return nil, errors.New("session_id is required")
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, errors.New("query is required")
	}

	mode, cleanQuery := extractMode(req.Query)

	history, err := s.memory.Load(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	rewritten, err := s.rewriter.Rewrite(ctx, cleanQuery, history)
	if err != nil {
		return nil, err
	}

	embeddingVec, err := s.embedder.Embed(ctx, rewritten)
	if err != nil {
		return nil, err
	}

	docs, err := s.retriever.Retrieve(ctx, rewritten, embeddingVec, req.Filters)
	if err != nil {
		return nil, err
	}
	fmt.Println("==== RETRIEVED DOCS ====")
	fmt.Println("Query:", rewritten)
	fmt.Println("Num docs:", len(docs))
	// for i, d := range docs {
	//      fmt.Printf("[%d] %s\n%s\n\n", i, d.Source, d.Content)
	// }

	prompt := buildPrompt(mode, rewritten, docs, history)

	rawStream, err := s.llm.Stream(ctx, prompt)
	if err != nil {
		return nil, err
	}

	out := make(chan string)

	go func() {
		defer close(out)

		var fullAnswer strings.Builder

		_ = s.memory.Save(ctx, req.SessionID, memory.Message{
			Role:    "user",
			Content: cleanQuery,
		})

		for token := range rawStream {
			fullAnswer.WriteString(token)

			select {
			case <-ctx.Done():
				return
			case out <- token:
			}
		}

		if len(docs) > 0 {
			citation := " [1]"
			select {
			case <-ctx.Done():
				return
			case out <- citation:
			}
			fullAnswer.WriteString(citation)
		}

		_ = s.memory.Save(ctx, req.SessionID, memory.Message{
			Role:    "assistant",
			Content: fullAnswer.String(),
		})
	}()

	return out, nil
}

func extractMode(query string) (string, string) {
	query = strings.TrimSpace(query)
	if strings.HasPrefix(query, "[") {
		end := strings.Index(query, "]")
		if end > 0 {
			mode := strings.TrimSpace(query[1:end])
			cleanQuery := strings.TrimSpace(query[end+1:])
			if mode != "" && cleanQuery != "" {
				return mode, cleanQuery
			}
		}
	}

	return "Recruiter", query
}

func buildPrompt(mode string, query string, docs []Document, history []memory.Message) string {
	var b strings.Builder

	b.WriteString("You are Charmaine Cat, Charmaine's personal assistant.\n")
	b.WriteString("You answer recruiters, HR, hiring managers, and interviewers on Charmaine's behalf.\n")
	b.WriteString("When the user says 'she' or 'her', they are referring to Charmaine.\n")
	b.WriteString("Use the retrieved context as your primary source of truth.\n")
	b.WriteString("Do not ask the user to paste context. If context is weak, say what you can based on the available retrieved material and mention that the knowledge base may need more data.\n")
	b.WriteString("Keep answers professional, specific, and accurate.\n")
	b.WriteString("When useful, answer directly first, then add one short supporting detail.\n")
	b.WriteString(getModeInstruction(mode))
	b.WriteString("\n")

	if len(history) > 0 {
		b.WriteString("Conversation history:\n")
		for _, msg := range history {
			b.WriteString("- ")
			b.WriteString(msg.Role)
			b.WriteString(": ")
			b.WriteString(msg.Content)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("User mode:\n")
	b.WriteString(mode)
	b.WriteString("\n\n")

	b.WriteString("User question:\n")
	b.WriteString(query)
	b.WriteString("\n\n")

	b.WriteString("Retrieved knowledge base context:\n")
	if len(docs) == 0 {
		b.WriteString("No relevant chunks were retrieved from the knowledge base for this question.\n")
	} else {
		for i, doc := range docs {
			b.WriteString("\n[")
			b.WriteString(doc.Source)
			b.WriteString(" / ")
			b.WriteString(doc.ID)
			b.WriteString("]\n")
			b.WriteString(doc.Content)
			if i < len(docs)-1 {
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n\nAnswer as Charmaine Cat:\n")

	return b.String()
}

func getModeInstruction(mode string) string {
	switch mode {
	case "Technical Interviewer":
		return "Mode instruction: Answer with more technical depth. Mention frameworks, architecture, implementation details, tradeoffs, and concrete engineering examples when the context supports them.\n"
	case "Hiring Manager":
		return "Mode instruction: Focus on ownership, impact, collaboration, reliability, delivery, and role fit. Keep the answer concise and outcome-oriented.\n"
	case "HR":
		return "Mode instruction: Use a clear, professional, non-technical tone. Focus on eligibility, communication, availability, experience summary, and fit. Avoid unnecessary jargon.\n"
	case "Resume Reviewer":
		return "Mode instruction: Focus on resume-style evidence: skills, projects, years of experience, tools, responsibilities, and accomplishments. Be concise and structured.\n"
	default:
		return "Mode instruction: Answer in a recruiter-friendly way. Keep it concise, specific, and focused on relevant skills, experience, and fit.\n"
	}
}

func appendCitationHint(answer string, docs []Document) string {
	answer = strings.TrimSpace(answer)
	if answer == "" || len(docs) == 0 {
		return answer
	}

	if strings.Contains(answer, "[1]") {
		return answer
	}

	return answer + " [1]"
}
