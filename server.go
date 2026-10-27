package main

import (
	"crypto/rand"
	"net/http"
	"time"

	"github.com/devthenet-labs/beatbattle-api/internal/battle"
	"github.com/devthenet-labs/beatbattle-api/internal/httpapi"
	"github.com/devthenet-labs/beatbattle-api/internal/memstore"
)

// newHandler wires the production adapters (memory store and queue, the wall
// clock, crypto/rand) into the /api handler.
func newHandler(revision string) http.Handler {
	return newHandlerWith(battle.NewService(memstore.NewStore(), memstore.NewBattles(), memstore.NewQueue(), time.Now, rand.Reader), revision)
}

// newHandlerWith serves /api over svc, behind the security headers.
func newHandlerWith(svc *battle.Service, revision string) http.Handler {
	return withSecurityHeaders(httpapi.New(svc, revision))
}

// withSecurityHeaders sets the headers every response carries.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
