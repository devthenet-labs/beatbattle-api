package battle

import (
	"errors"
	"time"
)

// Limits of the v0 skeleton.
const (
	MaxPatterns    = 1000
	MaxRenders     = 10000
	MaxBattles     = 1000
	DefaultLimit   = 20
	MaxLimit       = 100
	LeaderboardTop = 10
)

// Errors the ports and the service report.
var (
	ErrNotFound  = errors.New("not found")
	ErrExists    = errors.New("id already exists")
	ErrStoreFull = errors.New("store full")
	ErrQueueFull = errors.New("render queue full")
)

// Store keeps patterns. Implementations must be safe for concurrent use and
// must not share slices or maps with the patterns they accept or return.
type Store interface {
	// Add stores p; ErrExists for a taken id, ErrStoreFull at MaxPatterns.
	Add(p Pattern) error
	// Get returns the pattern, or ErrNotFound.
	Get(id string) (Pattern, error)
	// Vote adds one vote and returns the new count, or ErrNotFound.
	Vote(id string) (int, error)
	// List returns every pattern in insertion order (oldest first).
	List() []Pattern
}

// Battle is a stored head-to-head matchup between two patterns, in the
// shared JSON shape.
type Battle struct {
	ID         string    `json:"id"`
	PatternAID string    `json:"patternAId"`
	PatternBID string    `json:"patternBId"`
	VotesA     int       `json:"votesA"`
	VotesB     int       `json:"votesB"`
	CreatedAt  time.Time `json:"createdAt"`
}

// BattleStore keeps battles. Implementations must be safe for concurrent use
// and must not share slices or maps with the battles they accept or return.
type BattleStore interface {
	// Add stores b; ErrExists for a taken id, ErrStoreFull at MaxBattles.
	Add(b Battle) error
	// Get returns the battle, or ErrNotFound.
	Get(id string) (Battle, error)
	// Vote increments VotesA or VotesB (side is exactly "a" or "b") and
	// returns the updated battle, or ErrNotFound for an unknown id.
	Vote(id, side string) (Battle, error)
	// List returns every battle in insertion order (oldest first).
	List() []Battle
}

// RenderStatus is where a render request is in its life.
type RenderStatus string

// StatusPending is the only status in v0: nothing completes a job yet.
const StatusPending RenderStatus = "pending"

// RenderJob is one render request.
type RenderJob struct {
	RenderID  string       `json:"renderId"`
	PatternID string       `json:"patternId"`
	Status    RenderStatus `json:"status"`
	CreatedAt time.Time    `json:"-"`
}

// RenderQueue records render requests for a worker that comes later.
type RenderQueue interface {
	// Enqueue records job; ErrExists for a taken id, ErrQueueFull when full.
	Enqueue(job RenderJob) error
	// Get returns the job, or ErrNotFound.
	Get(renderID string) (RenderJob, error)
}
