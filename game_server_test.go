package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJobDecision(t *testing.T) {
	for _, tc := range []struct {
		kind, text, want string
		status           int
	}{
		{"player_asset", "silver shield", "item description", 200},
		{"live_event", "weekend raid", "event announcement", 200},
		{"moderation_queue", "player chat", "allow or review", 200},
		{"unknown", "hello", "kind must be", 400},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			s := server{chat: func(_ context.Context, prompt string) (string, error) {
				if !strings.Contains(prompt, tc.want) {
					t.Errorf("prompt = %q", prompt)
				}
				return "accepted", nil
			}}
			body, _ := json.Marshal(job{Kind: tc.kind, Text: tc.text})
			r := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(string(body)))
			w := httptest.NewRecorder()
			s.routes().ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
			}
			if tc.status == 200 && !strings.Contains(w.Body.String(), "accepted") {
				t.Fatal(w.Body.String())
			}
		})
	}
}

func TestAccountEnvelopeAndRetry(t *testing.T) {
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != "/v1/account/usage/timeseries" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Retry-After", "0")
		if requests == 1 {
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"RATE_LIMIT","message":"retry"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"points":[]},"metadata":{}}`))
	}))
	defer upstream.Close()
	c := accountClient{key: "test-key", base: upstream.URL, http: upstream.Client(), backoff: func(time.Duration) {}}
	if _, err := c.usage(context.Background()); err != nil || requests != 2 {
		t.Fatalf("requests=%d err=%v", requests, err)
	}
}

func TestBusinessRejection(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"rejected","message":"invalid period"}}`))
	}))
	defer upstream.Close()
	c := accountClient{key: "test-key", base: upstream.URL, http: upstream.Client(), backoff: func(time.Duration) {}}
	s := server{account: c}
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/budget", strings.NewReader(`{"hard_cap_usd":25,"period":"monthly"}`)))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "invalid period") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
