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
