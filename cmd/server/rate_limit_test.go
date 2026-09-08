package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRateLimitIsolatedByForwardedClient(t *testing.T) {
	limiter := newRateLimiter(2, time.Minute)
	handler := limiter.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := func(forwardedFor string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("X-Forwarded-For", forwardedFor)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	for i := 0; i < 2; i++ {
		if rr := request("203.0.113.10, 10.0.0.1"); rr.Code != http.StatusNoContent {
			t.Fatalf("request %d unexpectedly limited: %d", i+1, rr.Code)
		}
	}
	limited := request("203.0.113.10, 10.0.0.1")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", limited.Code)
	}
	retryAfter, err := strconv.Atoi(limited.Header().Get("Retry-After"))
	if err != nil || retryAfter < 1 || retryAfter > 60 {
		t.Fatalf("invalid Retry-After %q", limited.Header().Get("Retry-After"))
	}
	if rr := request("203.0.113.11, 10.0.0.1"); rr.Code != http.StatusNoContent {
		t.Fatalf("one client exhausted another client's quota: %d", rr.Code)
	}
}

func TestRateLimitUsesCredentialWithoutForwardedAddress(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)
	handler := limiter.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := func(token string) int {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	if got := request("user-a"); got != http.StatusNoContent {
		t.Fatalf("first user-a request: got %d", got)
	}
	if got := request("user-a"); got != http.StatusTooManyRequests {
		t.Fatalf("second user-a request: got %d", got)
	}
	if got := request("user-b"); got != http.StatusNoContent {
		t.Fatalf("user-b shared user-a quota: got %d", got)
	}
}
