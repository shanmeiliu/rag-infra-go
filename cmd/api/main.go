package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shanmeiliu/rag-infra-go/internal/chat"
	"github.com/shanmeiliu/rag-infra-go/internal/db"
	"github.com/shanmeiliu/rag-infra-go/internal/memory"
	"github.com/shanmeiliu/rag-infra-go/internal/providers"
	"github.com/shanmeiliu/rag-infra-go/internal/retrieval"
	"github.com/shanmeiliu/rag-infra-go/internal/rewrite"
	"github.com/shanmeiliu/rag-infra-go/internal/transport"
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
