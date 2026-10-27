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
	return battle.NewService(memstore.NewStore(), memstore.NewBattles(), memstore.NewQueue(), c.now, bytes.NewReader(random)), c
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

func TestCreateBattle(t *testing.T) {
	svc, _ := newService(t, []byte{1, 0, 0, 2, 0, 0, 0xab, 0xcd, 0xef})
	a, _ := svc.Create(input("a"))
	b, _ := svc.Create(input("b"))

	v, err := svc.CreateBattle(battle.BattleInput{PatternAID: a.ID, PatternBID: b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if v.ID != "b_abcdef" || v.PatternA.ID != a.ID || v.PatternB.ID != b.ID || v.VotesA != 0 || v.VotesB != 0 {
		t.Errorf("battle = %+v", v)
	}

	if _, err := svc.CreateBattle(battle.BattleInput{PatternBID: b.ID}); !errors.Is(err, battle.ErrInvalid) {
		t.Errorf("missing patternAId err = %v", err)
	}
	if _, err := svc.CreateBattle(battle.BattleInput{PatternAID: a.ID}); !errors.Is(err, battle.ErrInvalid) {
		t.Errorf("missing patternBId err = %v", err)
	}
	if _, err := svc.CreateBattle(battle.BattleInput{PatternAID: a.ID, PatternBID: a.ID}); !errors.Is(err, battle.ErrInvalid) {
		t.Errorf("same id err = %v", err)
	}
	if _, err := svc.CreateBattle(battle.BattleInput{PatternAID: "p_nope", PatternBID: b.ID}); !errors.Is(err, battle.ErrNotFound) {
		t.Errorf("unknown patternAId err = %v", err)
	}
	if _, err := svc.CreateBattle(battle.BattleInput{PatternAID: a.ID, PatternBID: "p_nope"}); !errors.Is(err, battle.ErrNotFound) {
		t.Errorf("unknown patternBId err = %v", err)
	}
}

func TestGetBattle(t *testing.T) {
	svc, _ := newService(t, []byte{1, 0, 0, 2, 0, 0, 3, 0, 0})
	a, _ := svc.Create(input("a"))
	b, _ := svc.Create(input("b"))
	created, err := svc.CreateBattle(battle.BattleInput{PatternAID: a.ID, PatternBID: b.ID})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetBattle(created.ID)
	if err != nil || got.ID != created.ID || got.PatternA.ID != a.ID || got.PatternB.ID != b.ID {
		t.Errorf("GetBattle = %+v, %v", got, err)
	}
	if _, err := svc.GetBattle("b_nope"); !errors.Is(err, battle.ErrNotFound) {
		t.Errorf("unknown battle err = %v", err)
	}
}

func TestListBattlesNewestFirstAndLimits(t *testing.T) {
	random := make([]byte, 0, 3*160)
	for i := 0; i < 160; i++ {
		random = append(random, byte(i), 0, 0)
	}
	svc, _ := newService(t, random)
	a, _ := svc.Create(input("a"))
	b, _ := svc.Create(input("b"))
	var ids []string
	for i := 0; i < 150; i++ {
		v, err := svc.CreateBattle(battle.BattleInput{PatternAID: a.ID, PatternBID: b.ID})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, v.ID)
	}
	cases := []struct{ limit, want int }{{1, 1}, {20, 20}, {100, 100}, {500, 100}, {0, 1}}
	for _, tc := range cases {
		if got := len(svc.ListBattles(tc.limit)); got != tc.want {
			t.Errorf("ListBattles(%d) = %d items, want %d", tc.limit, got, tc.want)
		}
	}
	if first := svc.ListBattles(2); first[0].ID != ids[len(ids)-1] {
		t.Errorf("newest = %s, want %s", first[0].ID, ids[len(ids)-1])
	}
}

func TestVoteBattle(t *testing.T) {
	svc, _ := newService(t, []byte{1, 0, 0, 2, 0, 0, 3, 0, 0})
	a, _ := svc.Create(input("a"))
	b, _ := svc.Create(input("b"))
	created, err := svc.CreateBattle(battle.BattleInput{PatternAID: a.ID, PatternBID: b.ID})
	if err != nil {
		t.Fatal(err)
	}

	v, err := svc.VoteBattle(created.ID, "a")
	if err != nil || v.VotesA != 1 || v.VotesB != 0 {
		t.Fatalf("VoteBattle a = %+v, %v", v, err)
	}
	v, err = svc.VoteBattle(created.ID, "b")
	if err != nil || v.VotesA != 1 || v.VotesB != 1 {
		t.Fatalf("VoteBattle b = %+v, %v", v, err)
	}
	v, err = svc.VoteBattle(created.ID, "a")
	if err != nil || v.VotesA != 2 || v.VotesB != 1 {
		t.Fatalf("VoteBattle a again = %+v, %v", v, err)
	}

	gotA, _ := svc.Get(a.ID)
	gotB, _ := svc.Get(b.ID)
	if gotA.Votes != 2 || gotB.Votes != 1 {
		t.Errorf("pattern votes = a:%d b:%d, want a:2 b:1", gotA.Votes, gotB.Votes)
	}
	board := svc.Leaderboard()
	if len(board) == 0 || board[0].ID != a.ID {
		t.Errorf("leaderboard top = %+v, want %s first", board, a.ID)
	}

	if _, err := svc.VoteBattle("b_nope", "a"); !errors.Is(err, battle.ErrNotFound) {
		t.Errorf("unknown battle err = %v", err)
	}
}
