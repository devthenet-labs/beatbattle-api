package battle

import (
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"time"
)

// Service is the use-case layer over the ports.
type Service struct {
	store   Store
	battles BattleStore
	queue   RenderQueue
	now     func() time.Time
	random  io.Reader
}

// NewService wires the core. now and random are injected; production passes
// time.Now and crypto/rand.Reader.
func NewService(store Store, battles BattleStore, queue RenderQueue, now func() time.Time, random io.Reader) *Service {
	return &Service{store: store, battles: battles, queue: queue, now: now, random: random}
}

const idAttempts = 5

// newID returns prefix plus 6 hex characters from the random source.
func (s *Service) newID(prefix string) (string, error) {
	var b [3]byte
	if _, err := io.ReadFull(s.random, b[:]); err != nil {
		return "", fmt.Errorf("draw an id: %w", err)
	}
	return prefix + hex.EncodeToString(b[:]), nil
}

// Create validates in and stores it as a new pattern.
func (s *Service) Create(in Input) (Pattern, error) {
	in, err := in.Validate()
	if err != nil {
		return Pattern{}, err
	}
	for range idAttempts {
		id, err := s.newID("p_")
		if err != nil {
			return Pattern{}, err
		}
		p := Pattern{
			ID: id, Name: in.Name, Author: in.Author, BPM: in.BPM, Swing: in.Swing,
			Tracks: in.Tracks, Rating: 1000, CreatedAt: s.now().UTC().Truncate(time.Second),
		}.Clone()
		switch err := s.store.Add(p); err {
		case nil:
			return s.withRemixCount(p), nil
		case ErrExists:
			continue
		default:
			return Pattern{}, err
		}
	}
	return Pattern{}, fmt.Errorf("draw an unused pattern id after %d attempts", idAttempts)
}

// Get returns one pattern.
func (s *Service) Get(id string) (Pattern, error) {
	p, err := s.store.Get(id)
	if err != nil {
		return Pattern{}, err
	}
	return s.withRemixCount(p), nil
}

// Remix validates in, checks the original exists, and stores a new pattern
// that copies the original's tracks, bpm and swing under the new name and
// author, with ParentID set to the original's id.
func (s *Service) Remix(id string, in RemixInput) (Pattern, error) {
	in, err := in.Validate()
	if err != nil {
		return Pattern{}, err
	}
	original, err := s.store.Get(id)
	if err != nil {
		return Pattern{}, err
	}
	for range idAttempts {
		newID, err := s.newID("p_")
		if err != nil {
			return Pattern{}, err
		}
		p := Pattern{
			ID: newID, ParentID: original.ID, Name: in.Name, Author: in.Author,
			BPM: original.BPM, Swing: original.Swing, Tracks: original.Tracks,
			Rating: 1000, CreatedAt: s.now().UTC().Truncate(time.Second),
		}.Clone()
		switch err := s.store.Add(p); err {
		case nil:
			return s.withRemixCount(p), nil
		case ErrExists:
			continue
		default:
			return Pattern{}, err
		}
	}
	return Pattern{}, fmt.Errorf("draw an unused pattern id after %d attempts", idAttempts)
}

// remixCounts counts patterns by non-empty ParentID, for use by both List
// and Leaderboard without re-scanning per item.
func remixCounts(patterns []Pattern) map[string]int {
	counts := make(map[string]int)
	for _, p := range patterns {
		if p.ParentID != "" {
			counts[p.ParentID]++
		}
	}
	return counts
}

// countRemixes scans the store once and counts patterns whose ParentID is
// id.
func (s *Service) countRemixes(id string) int {
	n := 0
	for _, p := range s.store.List() {
		if p.ParentID == id {
			n++
		}
	}
	return n
}

// withRemixCount returns p with RemixCount set from a fresh scan of the
// store.
func (s *Service) withRemixCount(p Pattern) Pattern {
	p.RemixCount = s.countRemixes(p.ID)
	return p
}

// Vote adds a vote and returns the new count.
func (s *Service) Vote(id string) (int, error) { return s.store.Vote(id) }

// List returns the newest patterns first. limit is clamped to 1..MaxLimit.
func (s *Service) List(limit int) []Pattern {
	limit = max(1, min(limit, MaxLimit))
	all := s.store.List()
	counts := remixCounts(all)
	out := make([]Pattern, 0, min(limit, len(all)))
	for i := len(all) - 1; i >= 0 && len(out) < limit; i-- {
		p := all[i]
		p.RemixCount = counts[p.ID]
		out = append(out, p)
	}
	return out
}

// Leaderboard returns the top patterns by votes.
func (s *Service) Leaderboard() []Pattern {
	all := s.store.List()
	counts := remixCounts(all)
	ranked := Rank(all, LeaderboardTop)
	for i := range ranked {
		ranked[i].RemixCount = counts[ranked[i].ID]
	}
	return ranked
}

// Rank orders patterns (given in insertion order) by votes, most first. Ties
// go to the earlier createdAt, then to the earlier insertion. It returns at
// most n patterns and does not modify its input.
func Rank(patterns []Pattern, n int) []Pattern {
	ranked := append([]Pattern{}, patterns...)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Votes != ranked[j].Votes {
			return ranked[i].Votes > ranked[j].Votes
		}
		return ranked[i].CreatedAt.Before(ranked[j].CreatedAt)
	})
	if len(ranked) > n {
		ranked = ranked[:n]
	}
	return ranked
}

// RankByRating orders patterns (given in insertion order) by rating,
// highest first. Ties go to the earlier createdAt, then to the earlier
// insertion. It returns at most n patterns and does not modify its input.
func RankByRating(patterns []Pattern, n int) []Pattern {
	ranked := append([]Pattern{}, patterns...)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Rating != ranked[j].Rating {
			return ranked[i].Rating > ranked[j].Rating
		}
		return ranked[i].CreatedAt.Before(ranked[j].CreatedAt)
	})
	if len(ranked) > n {
		ranked = ranked[:n]
	}
	return ranked
}

// Ratings returns the top patterns by rating, reduced to the ratings shape.
func (s *Service) Ratings() []RatingEntry {
	top := RankByRating(s.store.List(), RatingsTop)
	out := make([]RatingEntry, len(top))
	for i, p := range top {
		out[i] = RatingEntry{ID: p.ID, Name: p.Name, Author: p.Author, Rating: p.Rating}
	}
	return out
}

// view builds the served BattleView from a battle and its two already
// fetched patterns.
func (s *Service) view(b Battle, a, bside Pattern) BattleView {
	return BattleView{
		ID: b.ID, PatternA: s.withRemixCount(a), PatternB: s.withRemixCount(bside),
		VotesA: b.VotesA, VotesB: b.VotesB, CreatedAt: b.CreatedAt,
		Stakes: Stakes(a.Rating, bside.Rating),
	}
}

// CreateBattle validates in, checks both patterns exist, and stores a new
// battle between them.
func (s *Service) CreateBattle(in BattleInput) (BattleView, error) {
	in, err := in.Validate()
	if err != nil {
		return BattleView{}, err
	}
	a, err := s.store.Get(in.PatternAID)
	if err != nil {
		return BattleView{}, err
	}
	bside, err := s.store.Get(in.PatternBID)
	if err != nil {
		return BattleView{}, err
	}
	for range idAttempts {
		id, err := s.newID("b_")
		if err != nil {
			return BattleView{}, err
		}
		b := Battle{
			ID: id, PatternAID: in.PatternAID, PatternBID: in.PatternBID,
			CreatedAt: s.now().UTC().Truncate(time.Second),
		}
		switch err := s.battles.Add(b); err {
		case nil:
			return s.view(b, a, bside), nil
		case ErrExists:
			continue
		default:
			return BattleView{}, err
		}
	}
	return BattleView{}, fmt.Errorf("draw an unused battle id after %d attempts", idAttempts)
}

// GetBattle returns one battle, joined with its two patterns.
func (s *Service) GetBattle(id string) (BattleView, error) {
	b, err := s.battles.Get(id)
	if err != nil {
		return BattleView{}, err
	}
	a, err := s.store.Get(b.PatternAID)
	if err != nil {
		return BattleView{}, err
	}
	bside, err := s.store.Get(b.PatternBID)
	if err != nil {
		return BattleView{}, err
	}
	return s.view(b, a, bside), nil
}

// ListBattles returns the newest battles first. limit is clamped to
// 1..MaxLimit.
func (s *Service) ListBattles(limit int) []BattleView {
	limit = max(1, min(limit, MaxLimit))
	all := s.battles.List()
	out := make([]BattleView, 0, min(limit, len(all)))
	for i := len(all) - 1; i >= 0 && len(out) < limit; i-- {
		b := all[i]
		a, err := s.store.Get(b.PatternAID)
		if err != nil {
			continue
		}
		bside, err := s.store.Get(b.PatternBID)
		if err != nil {
			continue
		}
		out = append(out, s.view(b, a, bside))
	}
	return out
}

// VoteBattle adds a vote to one side of a battle. The voted pattern also
// gets a vote through Store.Vote, so it also rises on the main leaderboard.
// It also updates both patterns' Elo rating, using the ratings as they stood
// before this vote; a later vote on the same battle re-reads whatever
// ratings stand at that time, so repeated votes compose rather than reset.
func (s *Service) VoteBattle(id, side string) (BattleView, error) {
	b, err := s.battles.Get(id)
	if err != nil {
		return BattleView{}, err
	}
	winnerID, loserID := b.PatternAID, b.PatternBID
	if side == "b" {
		winnerID, loserID = b.PatternBID, b.PatternAID
	}
	winner, err := s.store.Get(winnerID)
	if err != nil {
		return BattleView{}, err
	}
	loser, err := s.store.Get(loserID)
	if err != nil {
		return BattleView{}, err
	}
	newWinnerRating, newLoserRating := Elo(winner.Rating, loser.Rating)
	if err := s.store.SetRating(winnerID, newWinnerRating); err != nil {
		return BattleView{}, err
	}
	if err := s.store.SetRating(loserID, newLoserRating); err != nil {
		return BattleView{}, err
	}
	if _, err := s.store.Vote(winnerID); err != nil {
		return BattleView{}, err
	}
	b, err = s.battles.Vote(id, side)
	if err != nil {
		return BattleView{}, err
	}
	a, err := s.store.Get(b.PatternAID)
	if err != nil {
		return BattleView{}, err
	}
	bside, err := s.store.Get(b.PatternBID)
	if err != nil {
		return BattleView{}, err
	}
	return s.view(b, a, bside), nil
}

// RequestRender queues a render of an existing pattern.
func (s *Service) RequestRender(patternID string) (RenderJob, error) {
	if _, err := s.store.Get(patternID); err != nil {
		return RenderJob{}, err
	}
	for range idAttempts {
		id, err := s.newID("r_")
		if err != nil {
			return RenderJob{}, err
		}
		job := RenderJob{RenderID: id, PatternID: patternID, Status: StatusPending, CreatedAt: s.now().UTC()}
		switch err := s.queue.Enqueue(job); err {
		case nil:
			return job, nil
		case ErrExists:
			continue
		default:
			return RenderJob{}, err
		}
	}
	return RenderJob{}, fmt.Errorf("draw an unused render id after %d attempts", idAttempts)
}

// Render returns a render job's status.
func (s *Service) Render(renderID string) (RenderJob, error) { return s.queue.Get(renderID) }
