package main

import (
	"context"
	"log"
	"net/http"

	"github.com/joho/godotenv"

	"github.com/shanmeiliu/rag-infra-go/internal/chat"
	"github.com/shanmeiliu/rag-infra-go/internal/db"
	"github.com/shanmeiliu/rag-infra-go/internal/memory"
	"github.com/shanmeiliu/rag-infra-go/internal/providers"
	"github.com/shanmeiliu/rag-infra-go/internal/retrieval"
	internalvector "github.com/shanmeiliu/rag-infra-go/internal/vectorstore"
	"github.com/shanmeiliu/rag-infra-go/internal/rewrite"
	"github.com/shanmeiliu/rag-infra-go/internal/transport"
	pkgvector "github.com/shanmeiliu/rag-infra-go/pkg/vectorstore"
)

func main() {
	_ = godotenv.Load()

	ctx := context.Background()

	cfg := db.ConfigFromEnv()
	postgresDB, err := db.Open(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer postgresDB.Close()

	if err := db.EnsureSchema(ctx, postgresDB); err != nil {
		log.Fatalf("failed to ensure schema: %v", err)
	}

	embedder := providers.NewMockEmbeddingClient()
	store := internalvector.NewPGVectorStore(postgresDB)

	if err := seedDemoData(ctx, embedder, store); err != nil {
		log.Fatalf("failed to seed demo data: %v", err)
	}

	retriever := retrieval.NewPGVectorRetriever(embedder, store, 5)
	rewriter := rewrite.NewSimpleRewriter()
	mem := memory.NewInMemoryStore()
	llmClient := providers.NewMockLLMClient()

	service := chat.NewService(chat.Dependencies{
		Rewriter:  rewriter,
		Retriever: retriever,
		Memory:    mem,
		LLM:       llmClient,
	})

	handler := transport.NewHTTPHandler(service)

	log.Println("Server running on :8080")
	if err := http.ListenAndServe(":8080", handler.Routes()); err != nil {
		log.Fatal(err)
	}
}

func seedDemoData(
	ctx context.Context,
	embedder *providers.MockEmbeddingClient,
	store *internalvector.PGVectorStore,
) error {
	chunks := []pkgvector.Chunk{
		{
			ChunkID:  "chunk-1",
			DocID:    "doc-rag-basics",
			Content:  "RAG stands for Retrieval-Augmented Generation. It retrieves relevant context before asking the language model to answer.",
			Metadata: map[string]any{"topic": "rag"},
		},
		{
			ChunkID:  "chunk-2",
			DocID:    "doc-hybrid-retrieval",
			Content:  "Hybrid retrieval combines lexical search and vector search to improve both precision and recall.",
			Metadata: map[string]any{"topic": "retrieval"},
		},
		{
			ChunkID:  "chunk-3",
			DocID:    "doc-pgvector",
			Content:  "pgvector enables vector similarity search directly inside PostgreSQL and supports operators such as cosine distance and L2 distance.",
			Metadata: map[string]any{"topic": "pgvector"},
		},
	}

	for i := range chunks {
		emb, err := embedder.Embed(ctx, chunks[i].Content)
		if err != nil {
			return err
		}
		chunks[i].Embedding = emb
	}

	return store.Upsert(ctx, chunks)
}