package chat

import (
	"context"
	"errors"
	"strings"

	"github.com/yourname/rag-infra-go/internal/memory"
	"github.com/yourname/rag-infra-go/pkg/llm"
)

type Rewriter interface {
	Rewrite(ctx context.Context, query string, history []memory.Message) (string, error)
}

type Retriever interface {
	Retrieve(ctx context.Context, query string) ([]Document, error)
}

type MemoryStore interface {
	Load(ctx context.Context, sessionID string) ([]memory.Message, error)
	Save(ctx context.Context, sessionID string, msg memory.Message) error
}

type Document struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Source  string `json:"source"`
}

type Dependencies struct {
	Rewriter  Rewriter
	Retriever Retriever
	Memory    MemoryStore
	LLM       llm.Client
}

type Service struct {
	rewriter  Rewriter
	retriever Retriever
	memory    MemoryStore
	llm       llm.Client
}

type Request struct {
	SessionID string `json:"session_id"`
	Query     string `json:"query"`
}

type Response struct {
	RewrittenQuery string     `json:"rewritten_query"`
	Documents      []Document `json:"documents"`
	Answer         string     `json:"answer"`
}

func NewService(dep Dependencies) *Service {
	return &Service{
		rewriter:  dep.Rewriter,
		retriever: dep.Retriever,
		memory:    dep.Memory,
		llm:       dep.LLM,
	}
}

func (s *Service) Ask(ctx context.Context, req Request) (*Response, error) {
	if strings.TrimSpace(req.SessionID) == "" {
		return nil, errors.New("session_id is required")
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, errors.New("query is required")
	}

	history, err := s.memory.Load(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	rewritten, err := s.rewriter.Rewrite(ctx, req.Query, history)
	if err != nil {
		return nil, err
	}

	docs, err := s.retriever.Retrieve(ctx, rewritten)
	if err != nil {
		return nil, err
	}

	prompt := buildPrompt(rewritten, docs, history)
	answer, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	if err := s.memory.Save(ctx, req.SessionID, memory.Message{
		Role:    "user",
		Content: req.Query,
	}); err != nil {
		return nil, err
	}

	if err := s.memory.Save(ctx, req.SessionID, memory.Message{
		Role:    "assistant",
		Content: answer,
	}); err != nil {
		return nil, err
	}

	return &Response{
		RewrittenQuery: rewritten,
		Documents:      docs,
		Answer:         answer,
	}, nil
}

func (s *Service) Stream(ctx context.Context, req Request) (<-chan string, error) {
	if strings.TrimSpace(req.SessionID) == "" {
		return nil, errors.New("session_id is required")
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, errors.New("query is required")
	}

	history, err := s.memory.Load(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	rewritten, err := s.rewriter.Rewrite(ctx, req.Query, history)
	if err != nil {
		return nil, err
	}

	docs, err := s.retriever.Retrieve(ctx, rewritten)
	if err != nil {
		return nil, err
	}

	prompt := buildPrompt(rewritten, docs, history)

	stream, err := s.llm.Stream(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

func buildPrompt(query string, docs []Document, history []memory.Message) string {
	var b strings.Builder
	b.WriteString("You are a helpful RAG assistant.\n\n")
	b.WriteString("Query:\n")
	b.WriteString(query)
	b.WriteString("\n\nRelevant context:\n")

	for i, doc := range docs {
		b.WriteString("- [")
		b.WriteString(doc.Source)
		b.WriteString("] ")
		b.WriteString(doc.Content)
		if i < len(docs)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
