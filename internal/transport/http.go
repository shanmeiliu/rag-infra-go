package transport

import (
	"encoding/json"
	"net/http"

	"github.com/shanmeiliu/rag-infra-go/internal/chat"
)

type Handler struct {
	svc *chat.Service
}

func NewHTTPHandler(s *chat.Service) *Handler {
	return &Handler{svc: s}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", h.chat)
	return mux
}

func (h *Handler) chat(w http.ResponseWriter, r *http.Request) {
	var req chat.Request
	json.NewDecoder(r.Body).Decode(&req)

	resp, _ := h.svc.Ask(r.Context(), req)

	json.NewEncoder(w).Encode(map[string]interface{}{
    "response": resp,
})
}