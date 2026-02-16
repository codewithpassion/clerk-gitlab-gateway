package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	issuer := os.Getenv("CLERK_ISSUER_URL")
	if issuer == "" {
		log.Fatal("CLERK_ISSUER_URL is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	gw := &Gateway{
		ClerkIssuerURL: issuer,
		HTTPClient:     http.DefaultClient,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/authorize", gw.HandleAuthorize)
	mux.HandleFunc("/oauth/token", gw.HandleToken)
	mux.HandleFunc("/api/v4/user", gw.HandleUser)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("clerk-gitlab-gateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
