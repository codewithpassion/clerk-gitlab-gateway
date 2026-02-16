package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandleAuthorize_RedirectsToClerk(t *testing.T) {
	clerk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("authorize should redirect, not proxy")
	}))
	defer clerk.Close()

	gw := &Gateway{ClerkIssuerURL: clerk.URL, HTTPClient: clerk.Client()}

	req := httptest.NewRequest("GET", "/oauth/authorize?client_id=abc&redirect_uri=http://mm/callback&response_type=code&state=xyz", nil)
	w := httptest.NewRecorder()

	gw.HandleAuthorize(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}

	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("bad location header: %v", err)
	}

	if !strings.HasPrefix(loc.String(), clerk.URL+"/oauth/authorize") {
		t.Errorf("expected redirect to clerk, got %s", loc.String())
	}

	if !strings.Contains(loc.Query().Get("scope"), "openid") {
		t.Error("expected scope to contain openid")
	}

	if loc.Query().Get("client_id") != "abc" {
		t.Error("expected client_id to be preserved")
	}
}

func TestHandleToken_ProxiesToClerk(t *testing.T) {
	clerk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "grant_type=authorization_code") {
			t.Error("expected grant_type in body")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok_123",
			"token_type":   "bearer",
			"expires_in":   3600,
		})
	}))
	defer clerk.Close()

	gw := &Gateway{ClerkIssuerURL: clerk.URL, HTTPClient: clerk.Client()}

	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {"authcode123"},
		"redirect_uri": {"http://mm/callback"},
	}
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	gw.HandleToken(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["access_token"] != "tok_123" {
		t.Errorf("expected access_token tok_123, got %v", resp["access_token"])
	}
	if resp["token_type"] != "Bearer" {
		t.Errorf("expected token_type Bearer, got %v", resp["token_type"])
	}
}

func TestHandleToken_RejectsGet(t *testing.T) {
	gw := &Gateway{ClerkIssuerURL: "http://unused", HTTPClient: http.DefaultClient}
	req := httptest.NewRequest("GET", "/oauth/token", nil)
	w := httptest.NewRecorder()

	gw.HandleToken(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleUser_ReturnsGitLabFormat(t *testing.T) {
	clerk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok_123" {
			t.Error("expected authorization header to be forwarded")
		}
		json.NewEncoder(w).Encode(ClerkUserInfo{
			Sub:              "user_2abc",
			Email:            "alice@example.com",
			EmailVerified:    true,
			Name:             "Alice Smith",
			PreferredUsername: "alice",
		})
	}))
	defer clerk.Close()

	gw := &Gateway{ClerkIssuerURL: clerk.URL, HTTPClient: clerk.Client()}

	req := httptest.NewRequest("GET", "/api/v4/user", nil)
	req.Header.Set("Authorization", "Bearer tok_123")
	w := httptest.NewRecorder()

	gw.HandleUser(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var user GitLabUser
	json.NewDecoder(w.Body).Decode(&user)

	if user.ID <= 0 {
		t.Errorf("expected positive ID, got %d", user.ID)
	}
	if user.Username != "alice" {
		t.Errorf("expected username alice, got %s", user.Username)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", user.Email)
	}
	if user.Name != "Alice Smith" {
		t.Errorf("expected name Alice Smith, got %s", user.Name)
	}
	if user.Login != "alice" {
		t.Errorf("expected login alice, got %s", user.Login)
	}
}

func TestHandleUser_FallsBackToEmailUsername(t *testing.T) {
	clerk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ClerkUserInfo{
			Sub:   "user_noname",
			Email: "bob@example.com",
			Name:  "Bob",
		})
	}))
	defer clerk.Close()

	gw := &Gateway{ClerkIssuerURL: clerk.URL, HTTPClient: clerk.Client()}

	req := httptest.NewRequest("GET", "/api/v4/user", nil)
	req.Header.Set("Authorization", "Bearer tok_456")
	w := httptest.NewRecorder()

	gw.HandleUser(w, req)

	var user GitLabUser
	json.NewDecoder(w.Body).Decode(&user)

	if user.Username != "bob" {
		t.Errorf("expected username bob (from email), got %s", user.Username)
	}
}

func TestHandleUser_MissingAuth(t *testing.T) {
	gw := &Gateway{ClerkIssuerURL: "http://unused", HTTPClient: http.DefaultClient}
	req := httptest.NewRequest("GET", "/api/v4/user", nil)
	w := httptest.NewRecorder()

	gw.HandleUser(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
