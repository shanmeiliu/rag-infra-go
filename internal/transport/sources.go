package transport

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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

func (h *SourcesHandler) Sync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, ok := parseSourceID(r.URL.Path, "/api/sources/", "/sync")
	if !ok {
		http.Error(w, "invalid source id", http.StatusBadRequest)
		return
	}

	src, err := h.svc.SyncSource(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "source synced",
		"source":  src,
	})
}

func (h *SourcesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, ok := parseSourceID(r.URL.Path, "/api/sources/", "")
	if !ok {
		http.Error(w, "invalid source id", http.StatusBadRequest)
		return
	}

	if err := h.svc.DeleteSource(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseSourceID(path, prefix, suffix string) (int64, bool) {
	if !strings.HasPrefix(path, prefix) {
		return 0, false
	}

	trimmed := strings.TrimPrefix(path, prefix)
	if suffix != "" {
		if !strings.HasSuffix(trimmed, suffix) {
			return 0, false
		}
		trimmed = strings.TrimSuffix(trimmed, suffix)
	}
	trimmed = strings.Trim(trimmed, "/")

	id, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, false
	}

	return id, true
}
