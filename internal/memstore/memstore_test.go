package memstore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/devthenet-labs/beatbattle-api/internal/battle"
)

func TestStoreLimitsAndDuplicates(t *testing.T) {
	s := NewStore()
	for i := 0; i < battle.MaxPatterns; i++ {
		if err := s.Add(battle.Pattern{ID: fmt.Sprint(i)}); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	if err := s.Add(battle.Pattern{ID: "0"}); !errors.Is(err, battle.ErrExists) {
		t.Errorf("duplicate err = %v", err)
	}
	if err := s.Add(battle.Pattern{ID: "extra"}); !errors.Is(err, battle.ErrStoreFull) {
		t.Errorf("full err = %v", err)
	}
	if l := s.List(); l[0].ID != "0" || l[len(l)-1].ID != "999" {
		t.Error("List is not in insertion order")
	}
}

func TestStoreSetRating(t *testing.T) {
	s := NewStore()
	if err := s.Add(battle.Pattern{ID: "p_1", Rating: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRating("p_1", 1016); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("p_1")
	if err != nil || got.Rating != 1016 {
		t.Fatalf("Get after SetRating = %+v, %v", got, err)
	}
	if err := s.SetRating("p_nope", 1000); !errors.Is(err, battle.ErrNotFound) {
		t.Errorf("SetRating unknown err = %v", err)
	}
}

func TestBattlesLimitsAndDuplicates(t *testing.T) {
	s := NewBattles()
	for i := 0; i < battle.MaxBattles; i++ {
		if err := s.Add(battle.Battle{ID: fmt.Sprint(i)}); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	if err := s.Add(battle.Battle{ID: "0"}); !errors.Is(err, battle.ErrExists) {
		t.Errorf("duplicate err = %v", err)
	}
	if err := s.Add(battle.Battle{ID: "extra"}); !errors.Is(err, battle.ErrStoreFull) {
		t.Errorf("full err = %v", err)
	}
	if l := s.List(); l[0].ID != "0" || l[len(l)-1].ID != fmt.Sprint(battle.MaxBattles-1) {
		t.Error("List is not in insertion order")
	}
}

func TestBattlesGetAndVote(t *testing.T) {
	s := NewBattles()
	if err := s.Add(battle.Battle{ID: "b_1"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("b_1")
	if err != nil || got.VotesA != 0 || got.VotesB != 0 {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := s.Get("b_nope"); !errors.Is(err, battle.ErrNotFound) {
		t.Errorf("get unknown err = %v", err)
	}

	v, err := s.Vote("b_1", "a")
	if err != nil || v.VotesA != 1 || v.VotesB != 0 {
		t.Fatalf("Vote a = %+v, %v", v, err)
	}
	v, err = s.Vote("b_1", "b")
	if err != nil || v.VotesA != 1 || v.VotesB != 1 {
		t.Fatalf("Vote b = %+v, %v", v, err)
	}
	if _, err := s.Vote("b_nope", "a"); !errors.Is(err, battle.ErrNotFound) {
		t.Errorf("vote unknown err = %v", err)
	}
}

func TestQueueLimitsAndDuplicates(t *testing.T) {
	q := NewQueue()
	for i := 0; i < battle.MaxRenders; i++ {
		if err := q.Enqueue(battle.RenderJob{RenderID: fmt.Sprint(i)}); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	if err := q.Enqueue(battle.RenderJob{RenderID: "0"}); !errors.Is(err, battle.ErrExists) {
		t.Errorf("duplicate err = %v", err)
	}
	if err := q.Enqueue(battle.RenderJob{RenderID: "extra"}); !errors.Is(err, battle.ErrQueueFull) {
		t.Errorf("full err = %v", err)
	}
}
