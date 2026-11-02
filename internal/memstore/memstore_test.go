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

func TestHistoryAppendAndList(t *testing.T) {
	h := NewHistory()
	if l := h.List("p_1"); l == nil || len(l) != 0 {
		t.Fatalf("List with nothing recorded = %v, want empty non-nil slice", l)
	}

	h.Append("p_1", battle.RatingHistoryEntry{BattleID: "b_1", Before: 1000, After: 1016})
	h.Append("p_1", battle.RatingHistoryEntry{BattleID: "b_2", Before: 1016, After: 1031})
	h.Append("p_2", battle.RatingHistoryEntry{BattleID: "b_1", Before: 1000, After: 984})

	got1 := h.List("p_1")
	if len(got1) != 2 || got1[0].BattleID != "b_1" || got1[1].BattleID != "b_2" {
		t.Errorf("List(p_1) = %+v, want [b_1 b_2] in insertion order", got1)
	}
	got2 := h.List("p_2")
	if len(got2) != 1 || got2[0].BattleID != "b_1" || got2[0].After != 984 {
		t.Errorf("List(p_2) = %+v, want one entry for b_1", got2)
	}

	got1[0].BattleID = "mutated"
	if again := h.List("p_1"); again[0].BattleID != "b_1" {
		t.Error("List returned a slice aliased with the stored one")
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
