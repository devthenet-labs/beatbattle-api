// Package httpapi is the thin HTTP adapter over the battle core, served under
// /api. Errors are JSON {"error": "<message>"}.
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/devthenet-labs/beatbattle-api/internal/battle"
)

// maxBody bounds every request body.
const maxBody = 16 << 10

// New returns the /api handler.
func New(svc *battle.Service, revision string, now func() time.Time) http.Handler {
	a := &api{svc: svc, revision: revision, limiter: newVoteLimiter(now)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", a.healthz)
	mux.HandleFunc("POST /api/patterns", a.createPattern)
	mux.HandleFunc("GET /api/patterns", a.listPatterns)
	mux.HandleFunc("GET /api/patterns/{id}", a.getPattern)
	mux.HandleFunc("POST /api/patterns/{id}/votes", a.vote)
	mux.HandleFunc("POST /api/patterns/{id}/remix", a.remixPattern)
	mux.HandleFunc("GET /api/leaderboard", a.leaderboard)
	mux.HandleFunc("GET /api/ratings", a.ratings)
	mux.HandleFunc("POST /api/battles", a.createBattle)
	mux.HandleFunc("GET /api/battles", a.listBattles)
	mux.HandleFunc("GET /api/battles/{id}", a.getBattle)
	mux.HandleFunc("POST /api/battles/{id}/votes", a.voteBattle)
	mux.HandleFunc("POST /api/patterns/{id}/renders", a.requestRender)
	mux.HandleFunc("GET /api/renders/{id}", a.getRender)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	return mux
}

type api struct {
	svc      *battle.Service
	revision string
	limiter  *voteLimiter
}

// rateLimitVote answers 429 and reports false when r's client (the
// X-Client-Id header, or "" when absent) has already voted voteLimit
// times in the last voteWindow. A refused request is not recorded by
// the limiter, so it never changes the vote count underneath it.
func (a *api) rateLimitVote(w http.ResponseWriter, r *http.Request) bool {
	ok, seconds := a.limiter.allow(r.Header.Get("X-Client-Id"))
	if ok {
		return true
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, http.StatusTooManyRequests, "too many votes")
	return false
}

func (a *api) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "revision": a.revision})
}

func (a *api) createPattern(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	in, err := battle.ParseInput(bytes.NewReader(body))
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := a.svc.Create(in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (a *api) listPatterns(w http.ResponseWriter, r *http.Request) {
	limit := battle.DefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = n
	}
	writeJSON(w, http.StatusOK, a.svc.List(limit))
}

func (a *api) getPattern(w http.ResponseWriter, r *http.Request) {
	p, err := a.svc.Get(r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *api) remixPattern(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	in, err := battle.ParseRemixInput(bytes.NewReader(body))
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := a.svc.Remix(r.PathValue("id"), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (a *api) vote(w http.ResponseWriter, r *http.Request) {
	if !a.rateLimitVote(w, r) {
		return
	}
	if _, ok := readBody(w, r); !ok {
		return
	}
	votes, err := a.svc.Vote(r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"votes": votes})
}

func (a *api) leaderboard(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.svc.Leaderboard())
}

func (a *api) ratings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.svc.Ratings())
}

func (a *api) createBattle(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	in, err := battle.ParseBattleInput(bytes.NewReader(body))
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := a.svc.CreateBattle(in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (a *api) listBattles(w http.ResponseWriter, r *http.Request) {
	limit := battle.DefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = n
	}
	writeJSON(w, http.StatusOK, a.svc.ListBattles(limit))
}

func (a *api) getBattle(w http.ResponseWriter, r *http.Request) {
	v, err := a.svc.GetBattle(r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *api) voteBattle(w http.ResponseWriter, r *http.Request) {
	if !a.rateLimitVote(w, r) {
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	in, err := battle.ParseBattleVoteInput(bytes.NewReader(body))
	if err != nil {
		fail(w, r, err)
		return
	}
	side, err := in.Validate()
	if err != nil {
		fail(w, r, err)
		return
	}
	before, _ := a.svc.GetBattle(r.PathValue("id"))
	v, err := a.svc.VoteBattle(r.PathValue("id"), side)
	if err != nil {
		fail(w, r, err)
		return
	}
	if (side == "a" && before.PatternA.Rating < before.PatternB.Rating) ||
		(side == "b" && before.PatternB.Rating < before.PatternA.Rating) {
		panic(fmt.Sprintf("upset bonus not implemented: battle %s side %s", v.ID, side))
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *api) requestRender(w http.ResponseWriter, r *http.Request) {
	if _, ok := readBody(w, r); !ok {
		return
	}
	job, err := a.svc.RequestRender(r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (a *api) getRender(w http.ResponseWriter, r *http.Request) {
	job, err := a.svc.Render(r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// readBody reads the whole body under the size limit, answering 413 itself.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		fail(w, r, err)
		return nil, false
	}
	return body, true
}

// fail maps an error onto a status code and a JSON body.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
	case errors.Is(err, battle.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, battle.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, battle.ErrStoreFull):
		writeError(w, http.StatusInsufficientStorage, "store full")
	case errors.Is(err, battle.ErrQueueFull):
		writeError(w, http.StatusInsufficientStorage, "render queue full")
	default:
		slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
