// beatbattle-api is the Beat Battle api: patterns, votes, the leaderboard and
// render requests under /api. It listens on one port, answers /api/healthz for
// readiness checks and writes nothing to disk, so it runs with a read-only root
// filesystem, as a maul preview runs it.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// defaultAddr is the one port the runtime image exposes; ADDR overrides it
// for local runs.
const defaultAddr = ":8080"

// commitSHA is set at build time (-ldflags -X main.commitSHA=...).
var commitSHA = "dev"

func main() {
	addr := defaultAddr
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{
		Addr:              addr,
		Handler:           newHandler(commitSHA),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			slog.Error("shutdown", "error", err)
		}
	}()
	slog.Info("listening", "address", addr, "revision", commitSHA)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("serve", "error", err)
		os.Exit(1)
	}
	<-shutdownDone
}
