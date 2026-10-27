// Package memstore holds the in-memory adapters for the battle ports: a
// mutex and maps. A database and a real queue replace them later.
package memstore

import (
	"sync"

	"github.com/devthenet-labs/beatbattle-api/internal/battle"
)

// Store is a battle.Store that keeps insertion order.
type Store struct {
	mu    sync.Mutex
	byID  map[string]int
	items []battle.Pattern
}

// NewStore returns an empty Store.
func NewStore() *Store { return &Store{byID: map[string]int{}} }

// Add implements battle.Store.
func (s *Store) Add(p battle.Pattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[p.ID]; ok {
		return battle.ErrExists
	}
	if len(s.items) >= battle.MaxPatterns {
		return battle.ErrStoreFull
	}
	s.byID[p.ID] = len(s.items)
	s.items = append(s.items, p.Clone())
	return nil
}

// Get implements battle.Store.
func (s *Store) Get(id string) (battle.Pattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.byID[id]
	if !ok {
		return battle.Pattern{}, battle.ErrNotFound
	}
	return s.items[i].Clone(), nil
}

// Vote implements battle.Store.
func (s *Store) Vote(id string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.byID[id]
	if !ok {
		return 0, battle.ErrNotFound
	}
	s.items[i].Votes++
	return s.items[i].Votes, nil
}

// List implements battle.Store.
func (s *Store) List() []battle.Pattern {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]battle.Pattern, len(s.items))
	for i, p := range s.items {
		out[i] = p.Clone()
	}
	return out
}

// Queue is a battle.RenderQueue that records jobs and never completes them.
type Queue struct {
	mu   sync.Mutex
	jobs map[string]battle.RenderJob
}

// NewQueue returns an empty Queue.
func NewQueue() *Queue { return &Queue{jobs: map[string]battle.RenderJob{}} }

// Enqueue implements battle.RenderQueue.
func (q *Queue) Enqueue(job battle.RenderJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.jobs[job.RenderID]; ok {
		return battle.ErrExists
	}
	if len(q.jobs) >= battle.MaxRenders {
		return battle.ErrQueueFull
	}
	q.jobs[job.RenderID] = job
	return nil
}

// Get implements battle.RenderQueue.
func (q *Queue) Get(renderID string) (battle.RenderJob, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	job, ok := q.jobs[renderID]
	if !ok {
		return battle.RenderJob{}, battle.ErrNotFound
	}
	return job, nil
}
