package transport

import (
	"encoding/json"
	"net/http"

	"github.com/shanmeiliu/rag-infra-go/internal/auth"
)

type AuthHandler struct {
	cfg auth.Config
}

func NewAuthHandler(cfg auth.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email != h.cfg.AdminEmail || req.Password != h.cfg.AdminPassword {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(h.cfg.JWTSecret, req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token": token,
		"user": map[string]any{
			"email": req.Email,
			"role":  "admin",
		},
	})
}
