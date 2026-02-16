package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.String(), r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

type ClerkUserInfo struct {
	Sub               string `json:"sub"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	Name              string `json:"name"`
	PreferredUsername  string `json:"preferred_username"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
}

type GitLabUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Login    string `json:"login"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

type Gateway struct {
	ClerkIssuerURL string
	HTTPClient     *http.Client
}

// HandleAuthorize redirects the user to Clerk's authorization endpoint.
func (g *Gateway) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	target, err := url.Parse(g.ClerkIssuerURL + "/oauth/authorize")
	if err != nil {
		http.Error(w, "bad issuer URL", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	// Ensure openid scopes are included
	scope := q.Get("scope")
	if !strings.Contains(scope, "openid") {
		q.Set("scope", "openid profile email")
	}
	target.RawQuery = q.Encode()

	log.Printf("authorize: redirecting to %s", target.String())
	http.Redirect(w, r, target.String(), http.StatusFound)
}

// HandleToken proxies the token exchange request to Clerk.
func (g *Gateway) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	clerkURL := g.ClerkIssuerURL + "/oauth/token"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, clerkURL, strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		http.Error(w, "upstream token request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed to read upstream response", http.StatusBadGateway)
		return
	}

	// Parse and ensure token_type is "Bearer" (Mattermost requires this)
	var tokenResp map[string]interface{}
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		// Not JSON -- pass through as-is
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
		return
	}

	tokenResp["token_type"] = "Bearer"

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	json.NewEncoder(w).Encode(tokenResp)
}

// HandleUser fetches the user's info from Clerk and returns it in GitLab format.
func (g *Gateway) HandleUser(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		http.Error(w, "missing authorization header", http.StatusUnauthorized)
		return
	}

	clerkURL := g.ClerkIssuerURL + "/oauth/userinfo"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, clerkURL, nil)
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", auth)

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		http.Error(w, "upstream userinfo request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, body), resp.StatusCode)
		return
	}

	var clerk ClerkUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&clerk); err != nil {
		http.Error(w, "failed to decode userinfo", http.StatusBadGateway)
		return
	}

	username := clerk.PreferredUsername
	if username == "" {
		username = strings.Split(clerk.Email, "@")[0]
	}

	user := GitLabUser{
		ID:       ClerkIDToNumeric(clerk.Sub),
		Username: username,
		Login:    username,
		Email:    clerk.Email,
		Name:     clerk.Name,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
