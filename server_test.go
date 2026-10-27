package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	handler := newHandler("abc123")
	cases := []struct {
		name, method, path string
		status             int
		contentType, body  string
	}{
		{"readiness", http.MethodGet, "/api/healthz", http.StatusOK, "application/json", `"status":"ok"`},
		{"leaderboard", http.MethodGet, "/api/leaderboard", http.StatusOK, "application/json", "[]"},
		{"unknown path", http.MethodGet, "/missing", http.StatusNotFound, "application/json", `"error"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("%s %s = %d, want %d", tc.method, tc.path, rec.Code, tc.status)
			}
			if got := rec.Header().Get("Content-Type"); got != tc.contentType {
				t.Errorf("Content-Type = %q, want %q", got, tc.contentType)
			}
			if !strings.Contains(rec.Body.String(), tc.body) {
				t.Errorf("body lacks %q:\n%s", tc.body, rec.Body.String())
			}
			for header, want := range map[string]string{
				"X-Content-Type-Options":  "nosniff",
				"Referrer-Policy":         "no-referrer",
				"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
			} {
				if got := rec.Header().Get(header); got != want {
					t.Errorf("%s = %q, want %q", header, got, want)
				}
			}
		})
	}
}

func TestReadinessReportsRevision(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler("abc123").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/healthz", nil))
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("readiness body is not JSON: %v", err)
	}
	if got["revision"] != "abc123" {
		t.Errorf("revision = %q, want abc123", got["revision"])
	}
}
