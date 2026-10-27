package battle_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/devthenet-labs/beatbattle-api/internal/battle"
	"github.com/devthenet-labs/beatbattle-api/internal/memstore"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newService(t *testing.T, random []byte) (*battle.Service, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 10, 26, 21, 0, 0, 123, time.UTC)}
	return battle.NewService(memstore.NewStore(), memstore.NewQueue(), c.now, bytes.NewReader(random)), c
}

func input(name string) battle.Input {
	on := make([]bool, battle.Steps)
	on[0] = true
	return battle.Input{Name: name, Author: "brvtl", BPM: 120, Tracks: map[string][]bool{
		"kick": on, "snare": make([]bool, battle.Steps), "hat": make([]bool, battle.Steps)}}
}

func TestCreateUsesInjectedClockAndRandom(t *testing.T) {
	svc, _ := newService(t, []byte{0x7f, 0x3a, 0x9c})
	p, err := svc.Create(input("  one  "))
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "p_7f3a9c" || p.Name != "one" || p.Votes != 0 {
		t.Errorf("pattern = %+v", p)
	}
	if want := time.Date(2026, 10, 26, 21, 0, 0, 0, time.UTC); !p.CreatedAt.Equal(want) {
		t.Errorf("createdAt = %v, want %v", p.CreatedAt, want)
	}
}

func TestCreateRetriesOnIDCollision(t *testing.T) {
	svc, _ := newService(t, []byte{1, 2, 3, 1, 2, 3, 4, 5, 6})
	a, _ := svc.Create(input("a"))
	b, err := svc.Create(input("b"))
	if err != nil || a.ID != "p_010203" || b.ID != "p_040506" {
		t.Fatalf("ids = %q, %q, err %v", a.ID, b.ID, err)
	}
}

func TestCreateRandomExhausted(t *testing.T) {
	svc, _ := newService(t, nil)
	if _, err := svc.Create(input("a")); err == nil {
		t.Fatal("want an error when the random source fails")
	}
}

func TestCreateStoreFull(t *testing.T) {
	random := make([]byte, 3*(battle.MaxPatterns+1))
	for i := 0; i < battle.MaxPatterns+1; i++ {
		random[3*i], random[3*i+1] = byte(i>>8), byte(i)
	}
	svc, _ := newService(t, random)
	for i := 0; i < battle.MaxPatterns; i++ {
		if _, err := svc.Create(input("x")); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if _, err := svc.Create(input("x")); !errors.Is(err, battle.ErrStoreFull) {
		t.Fatalf("err = %v, want ErrStoreFull", err)
	}
}

func TestStoredPatternIsNotAliased(t *testing.T) {
	svc, _ := newService(t, []byte{1, 1, 1})
	in := input("a")
	p, _ := svc.Create(in)
	in.Tracks["kick"][5] = true
	p.Tracks["kick"][6] = true
	got, _ := svc.Get(p.ID)
	if got.Tracks["kick"][5] || got.Tracks["kick"][6] {
		t.Error("stored pattern changed through a caller's slice")
	}
}

func TestListNewestFirstAndLimits(t *testing.T) {
	random := make([]byte, 0, 3*150)
	for i := 0; i < 150; i++ {
		random = append(random, byte(i), 0, 0)
	}
	svc, _ := newService(t, random)
	for i := 0; i < 150; i++ {
		if _, err := svc.Create(input("x")); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct{ limit, want int }{{1, 1}, {20, 20}, {100, 100}, {500, 100}, {0, 1}}
	for _, tc := range cases {
		if got := len(svc.List(tc.limit)); got != tc.want {
			t.Errorf("List(%d) = %d items, want %d", tc.limit, got, tc.want)
		}
	}
	if first := svc.List(2)[0]; first.ID != "p_950000" {
		t.Errorf("newest = %s, want p_950000", first.ID)
	}
}

func TestVoteAndLeaderboard(t *testing.T) {
	svc, c := newService(t, []byte{1, 0, 0, 2, 0, 0, 3, 0, 0})
	a, _ := svc.Create(input("a"))
	c.t = c.t.Add(time.Minute)
	b, _ := svc.Create(input("b"))
	c.t = c.t.Add(time.Minute)
	d, _ := svc.Create(input("d"))
	for _, id := range []string{d.ID, b.ID, a.ID, d.ID} {
		if _, err := svc.Vote(id); err != nil {
			t.Fatal(err)
		}
	}
	var ids []string
	for _, p := range svc.Leaderboard() {
		ids = append(ids, p.ID)
	}
	want := []string{d.ID, a.ID, b.ID} // d has 2; a and b tie on 1, a is earlier
	if len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Errorf("leaderboard = %v, want %v", ids, want)
	}
	if _, err := svc.Vote("p_nope"); !errors.Is(err, battle.ErrNotFound) {
		t.Errorf("vote unknown err = %v", err)
	}
}

func TestRank(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(id string, votes, minute int) battle.Pattern {
		return battle.Pattern{ID: id, Votes: votes, CreatedAt: base.Add(time.Duration(minute) * time.Minute)}
	}
	cases := []struct {
		name string
		in   []battle.Pattern
		n    int
		want string
	}{
		{"empty", nil, 10, ""},
		{"by votes", []battle.Pattern{mk("a", 1, 0), mk("b", 3, 1), mk("c", 2, 2)}, 10, "bca"},
		{"tie earlier first", []battle.Pattern{mk("late", 1, 5), mk("early", 1, 1)}, 10, "early,late"},
		{"same time keeps insertion", []battle.Pattern{mk("x", 1, 1), mk("y", 1, 1)}, 10, "xy"},
		{"truncates", []battle.Pattern{mk("a", 3, 0), mk("b", 2, 0), mk("c", 1, 0)}, 2, "ab"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := append([]battle.Pattern(nil), tc.in...)
			var got string
			for i, p := range battle.Rank(tc.in, tc.n) {
				if tc.name == "tie earlier first" && i > 0 {
					got += ","
				}
				got += p.ID
			}
			if got != tc.want {
				t.Errorf("Rank = %q, want %q", got, tc.want)
			}
			for i := range orig {
				if orig[i].ID != tc.in[i].ID {
					t.Fatal("Rank modified its input")
				}
			}
		})
	}
}

func TestRenderLifecycle(t *testing.T) {
	svc, _ := newService(t, []byte{1, 0, 0, 9, 9, 9})
	p, _ := svc.Create(input("a"))
	job, err := svc.RequestRender(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.RenderID != "r_090909" || job.PatternID != p.ID || job.Status != battle.StatusPending {
		t.Errorf("job = %+v", job)
	}
	got, err := svc.Render(job.RenderID)
	if err != nil || got.Status != battle.StatusPending {
		t.Errorf("Render = %+v, %v", got, err)
	}
	if _, err := svc.RequestRender("p_nope"); !errors.Is(err, battle.ErrNotFound) {
		t.Errorf("render unknown pattern err = %v", err)
	}
	if _, err := svc.Render("r_nope"); !errors.Is(err, battle.ErrNotFound) {
		t.Errorf("unknown render err = %v", err)
	}
}
