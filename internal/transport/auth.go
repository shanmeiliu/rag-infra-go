package transport

import (
	"encoding/json"
	"net/http"

	"github.com/shanmeiliu/rag-infra-go/internal/auth"
)

type AuthHandler struct {
	cfg auth.Config
	svc *auth.Service
}

func NewAuthHandler(cfg auth.Config, svc *auth.Service) *AuthHandler {
	return &AuthHandler{
		cfg: cfg,
		svc: svc,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ipAddress := r.RemoteAddr
	userAgent := r.UserAgent()

	user, sessionToken, err := h.svc.LoginWithPassword(
		r.Context(),
		req.Username,
		req.Password,
		&ipAddress,
		&userAgent,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.cfg.SessionTTL().Seconds()),
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user": map[string]any{
			"id":            user.ID,
			"username":      user.Username,
			"display_name":  user.DisplayName,
			"email":         user.Email,
			"role":          user.Role,
			"auth_provider": user.AuthProvider,
		},
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie(h.cfg.SessionCookieName)
	if err == nil && cookie.Value != "" {
		_ = h.svc.Logout(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user": map[string]any{
			"id":            user.ID,
			"username":      user.Username,
			"display_name":  user.DisplayName,
			"email":         user.Email,
			"role":          user.Role,
			"auth_provider": user.AuthProvider,
		},
	})
}
