package transport

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/shanmeiliu/rag-infra-go/internal/missingquestions"
)

type MissingQuestionsHandler struct {
	repo *missingquestions.Repository
}

func NewMissingQuestionsHandler(repo *missingquestions.Repository) *MissingQuestionsHandler {
	return &MissingQuestionsHandler{repo: repo}
}

func (h *MissingQuestionsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	items, err := h.repo.List(r.Context(), limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"items": items,
	})
}

func (h *MissingQuestionsHandler) Clear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.repo.Clear(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *MissingQuestionsHandler) DeleteByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, "/api/admin/missing-questions/")
	id, err := strconv.ParseInt(strings.Trim(raw, "/"), 10, 64)
	if err != nil {
		http.Error(w, "invalid missing question id", http.StatusBadRequest)
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
