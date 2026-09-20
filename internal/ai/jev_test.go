package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJevClientNotConfigured(t *testing.T) {
	client := NewJevClient(JevConfig{APIKey: ""})
	if client.Configured() {
		t.Fatal("expected client to be unconfigured without API key")
	}

	_, err := client.ClassifyCricketQuery(context.Background(), "Virat Kohli runs in IPL")
	if !errors.Is(err, ErrJevNotConfigured) {
		t.Fatalf("expected ErrJevNotConfigured, got %v", err)
	}
}

func TestJevClientClassifyCricketQuerySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		var req JevRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.State["question"] != "Top run scorers in IPL 2023" {
			t.Fatalf("unexpected question in state: %v", req.State["question"])
		}

		noulVal := 0.98
		intentChoice := "leading_run_scorers"
		leagueChoice := "IPL"
		phaseChoice := "all"

		resp := JevResponse{
			Answers: map[string]JevAnswer{
				"is_cricket": {
					Noul:       &noulVal,
					Confidence: 0.98,
				},
				"intent": {
					Choice:     &intentChoice,
					Confidence: 0.95,
				},
				"league": {
					Choice:     &leagueChoice,
					Confidence: 0.99,
				},
				"phase": {
					Choice:     &phaseChoice,
					Confidence: 0.90,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewJevClient(JevConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Timeout: 2 * time.Second,
	})

	classification, err := client.ClassifyCricketQuery(context.Background(), "Top run scorers in IPL 2023")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !classification.IsCricket {
		t.Fatal("expected IsCricket to be true")
	}
	if classification.Intent != "leading_run_scorers" {
		t.Fatalf("expected intent leading_run_scorers, got %s", classification.Intent)
	}
	if classification.League != "IPL" {
		t.Fatalf("expected league IPL, got %s", classification.League)
	}
	if classification.Phase != "all" {
		t.Fatalf("expected phase all, got %s", classification.Phase)
	}
	if classification.Confidence != 0.95 {
		t.Fatalf("expected confidence 0.95, got %f", classification.Confidence)
	}
}

func TestJevClientDetectsNonCricketPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		noulVal := 0.05
		intentChoice := "complex_query"
		leagueChoice := "unknown"
		phaseChoice := "all"

		resp := JevResponse{
			Answers: map[string]JevAnswer{
				"is_cricket": {
					Noul:       &noulVal,
					Confidence: 0.99,
				},
				"intent": {
					Choice:     &intentChoice,
					Confidence: 0.50,
				},
				"league": {
					Choice:     &leagueChoice,
					Confidence: 0.99,
				},
				"phase": {
					Choice:     &phaseChoice,
					Confidence: 0.90,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewJevClient(JevConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	classification, err := client.ClassifyCricketQuery(context.Background(), "What is the capital of France?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if classification.IsCricket {
		t.Fatal("expected IsCricket to be false for non-cricket query")
	}
}

func TestJevClientServerErrorFailsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewJevClient(JevConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	_, err := client.ClassifyCricketQuery(context.Background(), "Kohli runs in IPL")
	if !errors.Is(err, ErrJevUnavailable) {
		t.Fatalf("expected ErrJevUnavailable on 500 error, got %v", err)
	}
}
