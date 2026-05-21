package transport

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/shanmeiliu/rag-infra-go/internal/auth"
)

type AuthHandler struct {
	cfg         auth.Config
	svc         *auth.Service
	googleOAuth *auth.GoogleOAuthClient
}

func NewAuthHandler(cfg auth.Config, svc *auth.Service, googleOAuth *auth.GoogleOAuthClient) *AuthHandler {
	return &AuthHandler{
		cfg:         cfg,
		svc:         svc,
		googleOAuth: googleOAuth,
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

	result, err := h.svc.LoginWithPasswordMFAAware(
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

	if result.MFARequired {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mfa_required": true,
			"mfa_token":    result.MFAToken,
			"user":         serializeUser(result.User),
		})
		return
	}

	h.setSessionCookie(w, result.SessionToken)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user": serializeUser(result.User),
	})
}

func (h *AuthHandler) MFASetup(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	secret, otpauthURL, err := h.svc.BeginTOTPSetup(r.Context(), user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"secret":      secret,
		"otpauth_url": otpauthURL,
	})
}

func (h *AuthHandler) MFAConfirm(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Code string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.svc.ConfirmTOTP(r.Context(), user, req.Code); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) MFAVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MFAToken string `json:"mfa_token"`
		Code     string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ipAddress := r.RemoteAddr
	userAgent := r.UserAgent()

	user, sessionToken, err := h.svc.VerifyMFATOTP(
		r.Context(),
		req.MFAToken,
		req.Code,
		&ipAddress,
		&userAgent,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	h.setSessionCookie(w, sessionToken)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user": serializeUser(user),
	})
}

func (h *AuthHandler) MFADisable(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.svc.DisableMFA(r.Context(), user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Password    string  `json:"password"`
		DisplayName string  `json:"display_name"`
		Email       *string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email != nil {
		trimmed := strings.TrimSpace(*req.Email)
		req.Email = &trimmed
		if trimmed == "" {
			req.Email = nil
		}
	}

	ipAddress := r.RemoteAddr
	userAgent := r.UserAgent()

	user, sessionToken, err := h.svc.SignupRecruiterLocal(
		r.Context(),
		req.Password,
		req.DisplayName,
		req.Email,
		&ipAddress,
		&userAgent,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.setSessionCookie(w, sessionToken)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user": serializeUser(user),
	})
}

func (h *AuthHandler) GoogleStart(w http.ResponseWriter, r *http.Request) {
	url, err := h.googleOAuth.StartAuth(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	defer auth.ClearOAuthStateCookie(w, h.cfg.SecureCookies)

	if err := auth.ValidateOAuthStateCookie(r, r.URL.Query().Get("state")); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing oauth code", http.StatusBadRequest)
		return
	}

	ipAddress := r.RemoteAddr
	userAgent := r.UserAgent()

	googleUser, err := h.googleOAuth.ExchangeAndFetchUser(r.Context(), code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	user, sessionToken, err := h.svc.LoginWithGoogle(
		r.Context(),
		googleUser,
		&ipAddress,
		&userAgent,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	h.setSessionCookie(w, sessionToken)

	redirectBase := strings.TrimRight(h.cfg.FrontendPostLoginURL, "/")
	redirectURL := redirectBase + "/"
	if user.Role == "admin" {
		redirectURL = redirectBase + "/admin"
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
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
		"user": serializeUser(user),
	})
}

func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
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

	users, err := h.svc.ListUsers(r.Context(), limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]map[string]any, 0, len(users))
	for i := range users {
		u := users[i]
		out = append(out, serializeUser(&u))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"users": out,
	})
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, sessionToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.cfg.SessionTTL().Seconds()),
	})
}

func serializeUser(user *auth.User) map[string]any {
	return map[string]any{
		"id":                user.ID,
		"username":          user.Username,
		"display_name":      user.DisplayName,
		"email":             user.Email,
		"role":              user.Role,
		"auth_provider":     user.AuthProvider,
		"status":            user.Status,
		"created_at":        user.CreatedAt,
		"last_login_at":     user.LastLoginAt,
		"last_seen_at":      user.LastSeenAt,
		"expires_at":        user.ExpiresAt,
		"mfa_enabled":       user.MFAEnabled,
		"mfa_confirmed_at":  user.MFAConfirmedAt,
		"mfa_email_enabled": user.MFAEmailEnabled,
		"mfa_email":         user.MFAEmail,
	}
}
