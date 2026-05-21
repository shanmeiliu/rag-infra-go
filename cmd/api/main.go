package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/shanmeiliu/rag-infra-go/internal/auth"
	"github.com/shanmeiliu/rag-infra-go/internal/catprofile"
	"github.com/shanmeiliu/rag-infra-go/internal/chat"
	"github.com/shanmeiliu/rag-infra-go/internal/db"
	"github.com/shanmeiliu/rag-infra-go/internal/httpx"
	"github.com/shanmeiliu/rag-infra-go/internal/ingestion"
	"github.com/shanmeiliu/rag-infra-go/internal/memory"
	"github.com/shanmeiliu/rag-infra-go/internal/missingquestions"
	"github.com/shanmeiliu/rag-infra-go/internal/providers"
	"github.com/shanmeiliu/rag-infra-go/internal/rerank"
	"github.com/shanmeiliu/rag-infra-go/internal/retrieval"
	"github.com/shanmeiliu/rag-infra-go/internal/rewrite"
	"github.com/shanmeiliu/rag-infra-go/internal/sources"
	"github.com/shanmeiliu/rag-infra-go/internal/transport"
	internalvector "github.com/shanmeiliu/rag-infra-go/internal/vectorstore"
)

func loadEnv() {
	if envFile := os.Getenv("ENV_FILE"); envFile != "" {
		if err := godotenv.Load(envFile); err == nil {
			log.Printf("loaded env from %s", envFile)
			return
		}
	}

	candidates := []string{
		".env",
		"../.env",
		"../../.env",
	}

	for _, path := range candidates {
		if err := godotenv.Load(path); err == nil {
			log.Printf("loaded env from %s", path)
			return
		}
	}

	log.Println("No .env file found, using system environment variables")
}

func main() {
	loadEnv()

	ctx := context.Background()

	dbCfg := db.ConfigFromEnv()
	postgresDB, err := db.Open(ctx, dbCfg)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer postgresDB.Close()

	providerCfg := providers.LoadProviderConfig()

	embedder, err := providers.NewEmbeddingClient(providerCfg)
	if err != nil {
		log.Fatalf("failed to create embedding client: %v", err)
	}

	profile, err := providers.ResolveEmbeddingProfile(ctx, embedder, providerCfg)
	if err != nil {
		log.Fatalf("failed to resolve embedding profile: %v", err)
	}

	if err := db.EnsureBaseSchema(ctx, postgresDB); err != nil {
		log.Fatalf("failed to ensure base schema: %v", err)
	}
	if err := db.EnsureProfileSchema(ctx, postgresDB); err != nil {
		log.Fatalf("failed to ensure profile schema: %v", err)
	}
	if err := db.EnsureAuthSchema(ctx, postgresDB); err != nil {
		log.Fatalf("failed to ensure auth schema: %v", err)
	}
	if err := db.EnsureMFASchema(ctx, postgresDB); err != nil {
		log.Fatalf("failed to ensure MFA schema: %v", err)
	}
	if err := db.EnsureSourceSchema(ctx, postgresDB); err != nil {
		log.Fatalf("failed to ensure source schema: %v", err)
	}
	if err := db.EnsureCatProfileSchema(ctx, postgresDB); err != nil {
		log.Fatalf("failed to ensure cat profile schema: %v", err)
	}
	if err := db.EnsureEmbeddingTable(ctx, postgresDB, profile, providerCfg.EnableHNSWIndex); err != nil {
		log.Fatalf("failed to ensure embedding table: %v", err)
	}
	if err := db.UpsertEmbeddingProfile(ctx, postgresDB, profile, true); err != nil {
		log.Fatalf("failed to upsert embedding profile: %v", err)
	}
	if err := db.EnsureMissingQuestionsSchema(ctx, postgresDB); err != nil {
		log.Fatalf("failed to ensure missing questions schema: %v", err)
	}

	authCfg := auth.ConfigFromEnv()
	authRepo := auth.NewRepository(postgresDB)
	authSvc := auth.NewService(authCfg, authRepo)
	googleOAuth := auth.NewGoogleOAuthClient(authCfg)

	if err := authSvc.EnsureAdminUser(ctx); err != nil {
		log.Fatalf("failed to ensure admin user: %v", err)
	}

	llmClient := providers.NewOpenAIClient()
	store := internalvector.NewPGVectorStore(postgresDB, profile)

	rerankCfg := rerank.LoadConfig()
	rerankClient, err := rerank.NewClient(rerankCfg)
	if err != nil {
		log.Fatalf("failed to create reranker client: %v", err)
	}

	retriever := retrieval.NewHybridRetriever(store, postgresDB, 5, 0.7, rerankClient, rerankCfg.TopK)
	rewriter := rewrite.NewSimpleRewriter()
	memStore := memory.NewInMemoryStore()
	ingestionSvc := ingestion.NewService(embedder, store)
	missingQuestionsRepo := missingquestions.NewRepository(postgresDB)
	sourcesRepo := sources.NewRepository(postgresDB)
	sourcesSvc := sources.NewService(sourcesRepo, ingestionSvc, store, "./uploads")
	catProfileRepo := catprofile.NewRepository(postgresDB)
	chatSvc := chat.NewService(chat.Dependencies{
		Rewriter:      rewriter,
		Retriever:     retriever,
		Memory:        memStore,
		LLM:           llmClient,
		Embedder:      embedder,
		MissingLogger: missingQuestionsRepo,
	})

	handler := transport.NewHTTPHandler(
		chatSvc,
		ingestionSvc,
		store,
		authCfg,
		authSvc,
		googleOAuth,
		sourcesSvc,
		missingQuestionsRepo,
		catProfileRepo,
	)

	corsCfg := httpx.CORSConfigFromEnv()
	router := httpx.CORSMiddleware(corsCfg)(handler.Routes())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("app env: %s", os.Getenv("APP_ENV"))
	log.Printf("embedding provider: %s", profile.Provider)
	log.Printf("embedding model: %s", profile.Model)
	log.Printf("embedding dimension: %d", profile.Dimension)
	log.Printf("embedding table: %s", profile.TableName())
	log.Printf("server running on :%s", port)

	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
