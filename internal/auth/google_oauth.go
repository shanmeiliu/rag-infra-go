package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const oauthStateCookieName = "interview_copilot_oauth_state"

type GoogleOAuthClient struct {
	cfg        Config
	oauthConf  *oauth2.Config
	httpClient *http.Client
}

type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
	HD            string `json:"hd"`
}

func NewGoogleOAuthClient(cfg Config) *GoogleOAuthClient {
	return &GoogleOAuthClient{
		cfg: cfg,
		oauthConf: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes: []string{
				"openid",
				"email",
				"profile",
			},
			Endpoint: google.Endpoint,
		},
		httpClient: http.DefaultClient,
	}
}

func (c *GoogleOAuthClient) Enabled() bool {
	return c.cfg.GoogleClientID != "" && c.cfg.GoogleClientSecret != ""
}

func (c *GoogleOAuthClient) StartAuth(w http.ResponseWriter, r *http.Request) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("google oauth is not configured")
	}

	state, err := generateRandomURLToken(32)
	if err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	url := c.oauthConf.AuthCodeURL(state, oauth2.AccessTypeOnline)
	return url, nil
}

func (c *GoogleOAuthClient) ExchangeAndFetchUser(ctx context.Context, code string) (*GoogleUserInfo, error) {
	token, err := c.oauthConf.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	client := c.oauthConf.Client(ctx, token)
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch google userinfo: %s", resp.Status)
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	if userInfo.Sub == "" || userInfo.Email == "" {
		return nil, fmt.Errorf("google userinfo missing required fields")
	}

	if c.cfg.GoogleAllowedDomain != "" {
		emailDomain := ""
		if parts := strings.Split(userInfo.Email, "@"); len(parts) == 2 {
			emailDomain = parts[1]
		}
		if !strings.EqualFold(emailDomain, c.cfg.GoogleAllowedDomain) && !strings.EqualFold(userInfo.HD, c.cfg.GoogleAllowedDomain) {
			return nil, fmt.Errorf("google account domain is not allowed")
		}
	}

	return &userInfo, nil
}

func ValidateOAuthStateCookie(r *http.Request, incomingState string) error {
	cookie, err := r.Cookie(oauthStateCookieName)
	if err != nil {
		return fmt.Errorf("missing oauth state cookie")
	}
	if cookie.Value == "" || incomingState == "" || cookie.Value != incomingState {
		return fmt.Errorf("invalid oauth state")
	}
	return nil
}

func ClearOAuthStateCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func generateRandomURLToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
