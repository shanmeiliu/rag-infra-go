package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/shanmeiliu/rag-infra-go/internal/auth"
	"github.com/shanmeiliu/rag-infra-go/internal/chat"
	"github.com/shanmeiliu/rag-infra-go/internal/ingestion"
	"github.com/shanmeiliu/rag-infra-go/internal/sources"
	"github.com/shanmeiliu/rag-infra-go/pkg/vectorstore"
)

type Handler struct {
	chatSvc      *chat.Service
	ingestionSvc *ingestion.Service
	store        vectorstore.Store
	authCfg      auth.Config
	authSvc      *auth.Service
	googleOAuth  *auth.GoogleOAuthClient
	sourcesSvc   *sources.Service
}

func NewHTTPHandler(
	chatSvc *chat.Service,
	ingestionSvc *ingestion.Service,
	store vectorstore.Store,
	authCfg auth.Config,
	authSvc *auth.Service,
	googleOAuth *auth.GoogleOAuthClient,
	sourcesSvc *sources.Service,
) *Handler {
	return &Handler{
		chatSvc:      chatSvc,
		ingestionSvc: ingestionSvc,
		store:        store,
		authCfg:      authCfg,
		authSvc:      authSvc,
		googleOAuth:  googleOAuth,
		sourcesSvc:   sourcesSvc,
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	authHandler := NewAuthHandler(h.authCfg, h.authSvc, h.googleOAuth)
	requireAuth := auth.AuthMiddleware(h.authCfg, h.authSvc)
	sourcesHandler := NewSourcesHandler(h.sourcesSvc)

	mux.HandleFunc("/healthz", h.health)

	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.HandleFunc("/api/auth/signup", authHandler.Signup)
	mux.HandleFunc("/api/auth/google/start", authHandler.GoogleStart)
	mux.HandleFunc("/api/auth/google/callback", authHandler.GoogleCallback)
	mux.Handle("/api/auth/me", requireAuth(http.HandlerFunc(authHandler.Me)))
	mux.Handle("/api/auth/logout", requireAuth(http.HandlerFunc(authHandler.Logout)))

	mux.Handle("/api/admin/users", requireAuth(auth.AdminOnly(http.HandlerFunc(authHandler.ListUsers))))

	mux.Handle("/api/sources", requireAuth(auth.AdminOnly(http.HandlerFunc(sourcesHandler.List))))
	mux.Handle("/api/sources/upload", requireAuth(auth.AdminOnly(http.HandlerFunc(sourcesHandler.Upload))))
	mux.Handle("/api/sources/github", requireAuth(auth.AdminOnly(http.HandlerFunc(sourcesHandler.Github))))
	mux.Handle("/api/sources/", requireAuth(auth.AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			sourcesHandler.Delete(w, r)
			return
		}

		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/sync") {
			sourcesHandler.Sync(w, r)
			return
		}

		http.NotFound(w, r)
	}))))

	mux.Handle("/api/chat", requireAuth(http.HandlerFunc(h.chat)))
	mux.Handle("/api/chat/stream", requireAuth(http.HandlerFunc(h.chatStream)))

	mux.Handle("/api/ingest", requireAuth(auth.AdminOnly(http.HandlerFunc(h.ingest))))

	return mux
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) chat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req chat.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.chatSvc.Ask(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) chatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	var req chat.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	streamResult, err := h.chatSvc.StreamWithSources(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sourcesEvent, _ := json.Marshal(map[string]any{
		"type":            "sources",
		"documents":       streamResult.Documents,
		"mode":            streamResult.Mode,
		"rewritten_query": streamResult.RewrittenQuery,
	})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", sourcesEvent)
	flusher.Flush()

	for chunk := range streamResult.Tokens {
		data, _ := json.Marshal(map[string]string{
			"type":    "token",
			"content": chunk,
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	done, _ := json.Marshal(map[string]string{"type": "done"})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", done)
	flusher.Flush()
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Replace bool                   `json:"replace"`
		Chunks  []ingestion.InputChunk `json:"chunks"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Replace {
		if err := h.store.DeleteAll(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := h.ingestionSvc.Ingest(r.Context(), req.Chunks); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ingested": len(req.Chunks),
		"replace":  req.Replace,
	})
}
