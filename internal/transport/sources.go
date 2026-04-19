package transport

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/shanmeiliu/rag-infra-go/internal/auth"
	"github.com/shanmeiliu/rag-infra-go/internal/sources"
)

type SourcesHandler struct {
	svc *sources.Service
}

func NewSourcesHandler(svc *sources.Service) *SourcesHandler {
	return &SourcesHandler{svc: svc}
}

func (h *SourcesHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	items, err := h.svc.ListSources(r.Context(), limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"sources": items,
	})
}

func (h *SourcesHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	sourceType := r.FormValue("source_type")
	if sourceType == "" {
		sourceType = "document"
	}

	src, err := h.svc.HandleUploadedFile(r.Context(), user, header.Filename, sourceType, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "source uploaded",
		"source":  src,
	})
}

func (h *SourcesHandler) Github(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		RepoURL         string   `json:"repo_url"`
		Branch          string   `json:"branch"`
		IncludePatterns []string `json:"include_patterns"`
		SourceType      string   `json:"source_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SourceType == "" {
		req.SourceType = "github_repo"
	}

	src, err := h.svc.HandleGithubRepo(
		r.Context(),
		user,
		req.RepoURL,
		req.Branch,
		req.SourceType,
		req.IncludePatterns,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "github source ingested",
		"source":  src,
	})
}
