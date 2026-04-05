#!/bin/bash

# Project name
PROJECT_NAME="rag-infra-go"


cat > cmd/api/main.go <<'EOF'
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourname/rag-infra-go/internal/chat"
	"github.com/yourname/rag-infra-go/internal/db"
	"github.com/yourname/rag-infra-go/internal/memory"
	"github.com/yourname/rag-infra-go/internal/providers"
	"github.com/yourname/rag-infra-go/internal/retrieval"
	"github.com/yourname/rag-infra-go/internal/rewrite"
	"github.com/yourname/rag-infra-go/internal/transport"
)

func main() {
	cfg := db.ConfigFromEnv()

	ctx := context.Background()
	postgresDB, err := db.Open(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer postgresDB.Close()

	if err := db.EnsureSchema(ctx, postgresDB); err != nil {
		log.Fatalf("failed to ensure schema: %v", err)
	}

	retriever := retrieval.NewMockRetriever()
	rewriter := rewrite.NewSimpleRewriter()
	memStore := memory.NewInMemoryStore()
	llmClient := providers.NewMockLLMClient()

	chatService := chat.NewService(chat.Dependencies{
		Rewriter:  rewriter,
		Retriever: retriever,
		Memory:    memStore,
		LLM:       llmClient,
	})

	handler := transport.NewHTTPHandler(chatService)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
EOF

cat > internal/chat/service.go <<'EOF'
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
EOF

cat > internal/retrieval/retriever.go <<'EOF'
package retrieval

import (
	"context"

	"github.com/yourname/rag-infra-go/internal/chat"
)

type MockRetriever struct{}

func NewMockRetriever() *MockRetriever {
	return &MockRetriever{}
}

func (r *MockRetriever) Retrieve(ctx context.Context, query string) ([]chat.Document, error) {
	return []chat.Document{
		{
			ID:      "doc-1",
			Source:  "knowledge-base",
			Content: "RAG combines retrieval with generation by fetching relevant context before prompting the model.",
		},
		{
			ID:      "doc-2",
			Source:  "architecture-notes",
			Content: "A production RAG system usually includes query rewriting, retrieval, reranking, memory, and model fallback.",
		},
	}, nil
}
EOF

cat > internal/ingestion/pipeline.go <<'EOF'
package ingestion

import "context"

type Node interface {
	Name() string
	Run(ctx context.Context, state map[string]any) error
}

type Pipeline struct {
	nodes []Node
}

func NewPipeline(nodes ...Node) *Pipeline {
	return &Pipeline{nodes: nodes}
}

func (p *Pipeline) Run(ctx context.Context, state map[string]any) error {
	for _, node := range p.nodes {
		if err := node.Run(ctx, state); err != nil {
			return err
		}
	}
	return nil
}
EOF

cat > internal/memory/memory.go <<'EOF'
package memory

import (
	"context"
	"sync"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type InMemoryStore struct {
	mu    sync.RWMutex
	store map[string][]Message
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		store: make(map[string][]Message),
	}
}

func (s *InMemoryStore) Load(ctx context.Context, sessionID string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := s.store[sessionID]
	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out, nil
}

func (s *InMemoryStore) Save(ctx context.Context, sessionID string, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store[sessionID] = append(s.store[sessionID], msg)
	return nil
}
EOF

cat > internal/providers/llm.go <<'EOF'
package providers

import (
	"context"
	"strings"
	"time"

	"github.com/yourname/rag-infra-go/pkg/llm"
)

type MockLLMClient struct{}

func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{}
}

func (c *MockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "This is a mock answer generated from the retrieved context.", nil
}

func (c *MockLLMClient) Stream(ctx context.Context, prompt string) (<-chan string, error) {
	out := make(chan string)

	go func() {
		defer close(out)

		parts := strings.Split("This is a mock streamed answer generated from the retrieved context.", " ")
		for _, part := range parts {
			select {
			case <-ctx.Done():
				return
			case out <- part + " ":
				time.Sleep(60 * time.Millisecond)
			}
		}
	}()

	return out, nil
}

var _ llm.Client = (*MockLLMClient)(nil)
EOF

cat > internal/providers/embedding.go <<'EOF'
package providers

import "context"

type MockEmbeddingClient struct{}

func NewMockEmbeddingClient() *MockEmbeddingClient {
	return &MockEmbeddingClient{}
}

func (c *MockEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}
EOF

cat > internal/routing/router.go <<'EOF'
package routing

import "context"

type Route struct {
	Name string
}

type Router interface {
	Select(ctx context.Context) (Route, error)
}
EOF

cat > internal/rerank/reranker.go <<'EOF'
package rerank

import "context"

type Document struct {
	ID      string
	Content string
	Score   float64
}

type Reranker interface {
	Rerank(ctx context.Context, query string, docs []Document) ([]Document, error)
}
EOF

cat > internal/rewrite/rewrite.go <<'EOF'
package rewrite

import (
	"context"
	"strings"

	"github.com/yourname/rag-infra-go/internal/memory"
)

type SimpleRewriter struct{}

func NewSimpleRewriter() *SimpleRewriter {
	return &SimpleRewriter{}
}

func (r *SimpleRewriter) Rewrite(ctx context.Context, query string, history []memory.Message) (string, error) {
	return strings.TrimSpace(query), nil
}
EOF

cat > internal/trace/trace.go <<'EOF'
package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

func WithRequestID(ctx context.Context) context.Context {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return context.WithValue(ctx, requestIDKey, hex.EncodeToString(b))
}

func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}
EOF

cat > internal/transport/http.go <<'EOF'
package transport

import (
	"encoding/json"
	"net/http"

	"github.com/yourname/rag-infra-go/internal/chat"
	"github.com/yourname/rag-infra-go/internal/trace"
)

type HTTPHandler struct {
	chatService *chat.Service
}

func NewHTTPHandler(chatService *chat.Service) *HTTPHandler {
	return &HTTPHandler{chatService: chatService}
}

func (h *HTTPHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/api/chat", h.handleChat)
	mux.HandleFunc("/api/chat/stream", h.handleStream)
	return mux
}

func (h *HTTPHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HTTPHandler) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := trace.WithRequestID(r.Context())

	var req chat.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.chatService.Ask(ctx, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
EOF

cat > internal/transport/sse.go <<'EOF'
package transport

import (
	"fmt"
	"net/http"

	"github.com/yourname/rag-infra-go/internal/chat"
	"github.com/yourname/rag-infra-go/internal/trace"
)

func (h *HTTPHandler) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	ctx := trace.WithRequestID(r.Context())

	var req chat.Request
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	stream, err := h.chatService.Stream(ctx, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for token := range stream {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", token)
		flusher.Flush()
	}

	_, _ = fmt.Fprint(w, "event: done\ndata: [DONE]\n\n")
	flusher.Flush()
}

func decodeJSON(r *http.Request, dst any) error {
	return jsonNewDecoder(r).Decode(dst)
}
EOF

cat > internal/db/db.go <<'EOF'
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func ConfigFromEnv() Config {
	return Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "rag_platform"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

func (c Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		return nil, err
	}

	return db, nil
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
EOF

cat > internal/db/schema.go <<'EOF'
package db

import (
	"context"
	"database/sql"
)

func EnsureSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS vector;`,
		`CREATE TABLE IF NOT EXISTS documents (
			id BIGSERIAL PRIMARY KEY,
			doc_id TEXT UNIQUE NOT NULL,
			title TEXT,
			source TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS chunks (
			id BIGSERIAL PRIMARY KEY,
			chunk_id TEXT UNIQUE NOT NULL,
			doc_id TEXT NOT NULL,
			content TEXT NOT NULL,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			embedding VECTOR(1536),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_doc_id ON chunks(doc_id);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_metadata ON chunks USING GIN(metadata);`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
EOF

cat > pkg/llm/client.go <<'EOF'
package llm

import "context"

type Client interface {
	Generate(ctx context.Context, prompt string) (string, error)
	Stream(ctx context.Context, prompt string) (<-chan string, error)
}
EOF

cat > pkg/vectorstore/vectorstore.go <<'EOF'
package vectorstore

import "context"

type Chunk struct {
	ChunkID   string
	DocID     string
	Content   string
	Metadata  map[string]any
	Embedding []float32
}

type SearchResult struct {
	ChunkID  string
	DocID    string
	Content  string
	Metadata map[string]any
	Score    float64
}

type Store interface {
	Upsert(ctx context.Context, chunks []Chunk) error
	Search(ctx context.Context, embedding []float32, topK int, filters map[string]any) ([]SearchResult, error)
}
EOF

cat > pkg/embedding/embedding.go <<'EOF'
package embedding

import "context"

type Client interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
EOF

cat > pkg/pipeline/pipeline.go <<'EOF'
package pipeline

import "context"

type Node interface {
	Name() string
	Run(ctx context.Context, state map[string]any) error
}
EOF

cat > configs/config.yaml <<'EOF'
app:
  port: 8080
db:
  host: localhost
  port: 5432
  name: rag_platform
EOF

cat > scripts/dev.sh <<'EOF'
#!/bin/bash
set -e
go run ./cmd/api
EOF

cat > .env.example <<'EOF'
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=rag_platform
DB_SSLMODE=disable
OPENAI_API_KEY=
EOF

cat > .gitignore <<'EOF'
.env
bin/
dist/
*.log
EOF

cat > README.md <<'EOF'
# rag-infra-go

Production-oriented Agentic RAG backend implemented in Go.
EOF

cat > go.mod <<'EOF'
module github.com/yourname/rag-infra-go

go 1.24.0

require github.com/lib/pq v1.10.9
EOF

cat > internal/transport/json_compat.go <<'EOF'
package transport

import (
	"encoding/json"
	"net/http"
)

func jsonNewDecoder(r *http.Request) *json.Decoder {
	return json.NewDecoder(r.Body)
}
EOF

echo "Scaffold complete."
echo "Next steps:"
echo "1. go mod tidy"
echo "2. create Postgres database and enable pgvector"
echo "3. go run ./cmd/api"