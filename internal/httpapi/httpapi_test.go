package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devthenet-labs/beatbattle-api/internal/battle"
	"github.com/devthenet-labs/beatbattle-api/internal/httpapi"
	"github.com/devthenet-labs/beatbattle-api/internal/memstore"
)

// counterRand yields 000001, 000002, ... as three-byte draws.
type counterRand struct{ n int }

func (c *counterRand) Read(p []byte) (int, error) {
	c.n++
	p[0], p[1], p[2] = byte(c.n>>16), byte(c.n>>8), byte(c.n)
	return 3, nil
}

func newHandler() http.Handler {
	now := func() time.Time { return time.Date(2026, 10, 26, 21, 0, 0, 0, time.UTC) }
	svc := battle.NewService(memstore.NewStore(), memstore.NewBattles(), memstore.NewQueue(), now, &counterRand{})
	return httpapi.New(svc, "abc123")
}

const goodBody = `{"name":"Four","author":"brvtl","bpm":120,"swing":0,"tracks":{
 "kick":[true,false,false,false,true,false,false,false,true,false,false,false,true,false,false,false],
 "snare":[false,false,false,false,false,false,false,false,false,false,false,false,false,false,false,false],
 "hat":[false,false,false,false,false,false,false,false,false,false,false,false,false,false,false,false]}}`

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestRoutes(t *testing.T) {
	h := newHandler()
	cases := []struct {
		name, method, path, body string
		status                   int
		contains                 string
	}{
		{"healthz", "GET", "/api/healthz", "", 200, `"revision":"abc123"`},
		{"create", "POST", "/api/patterns", goodBody, 201, `"id":"p_000001"`},
		{"create ignores server fields", "POST", "/api/patterns",
			strings.Replace(goodBody, `"name"`, `"id":"p_evil","votes":99,"createdAt":"x","name"`, 1), 201, `"id":"p_000002"`},
		{"create invalid bpm", "POST", "/api/patterns", strings.Replace(goodBody, "120", "10", 1), 400, `"error"`},
		{"create unknown field", "POST", "/api/patterns", strings.Replace(goodBody, `"name"`, `"extra":1,"name"`, 1), 400, `"error"`},
		{"create not json", "POST", "/api/patterns", "nope", 400, `"error"`},
		{"create too large", "POST", "/api/patterns", `{"name":"` + strings.Repeat("a", 17<<10) + `"}`, 413, `"error"`},
		{"list", "GET", "/api/patterns", "", 200, `"p_000002"`},
		{"list limit", "GET", "/api/patterns?limit=1", "", 200, `"p_000002"`},
		{"list bad limit", "GET", "/api/patterns?limit=x", "", 400, `"error"`},
		{"list zero limit", "GET", "/api/patterns?limit=0", "", 400, `"error"`},
		{"get", "GET", "/api/patterns/p_000001", "", 200, `"name":"Four"`},
		{"get unknown", "GET", "/api/patterns/p_zzzzzz", "", 404, `"error"`},
		{"vote", "POST", "/api/patterns/p_000001/votes", "", 200, `{"votes":1}`},
		{"vote again", "POST", "/api/patterns/p_000001/votes", "", 200, `{"votes":2}`},
		{"vote unknown", "POST", "/api/patterns/p_zzzzzz/votes", "", 404, `"error"`},
		{"leaderboard", "GET", "/api/leaderboard", "", 200, `"votes":2`},
		{"render", "POST", "/api/patterns/p_000001/renders", "", 202, `"status":"pending"`},
		{"render unknown pattern", "POST", "/api/patterns/p_zzzzzz/renders", "", 404, `"error"`},
		{"render status", "GET", "/api/renders/r_000003", "", 200, `"patternId":"p_000001"`},
		{"render unknown", "GET", "/api/renders/r_zzzzzz", "", 404, `"error"`},
		{"unknown path", "GET", "/api/missing", "", 404, `"error"`},
		{"outside api", "GET", "/", "", 404, `"error"`},
		{"wrong method", "DELETE", "/api/patterns", "", 404, `"error"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(h, tc.method, tc.path, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("%s %s = %d, want %d: %s", tc.method, tc.path, rec.Code, tc.status, rec.Body)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q", got)
			}
			if !strings.Contains(rec.Body.String(), tc.contains) {
				t.Errorf("body lacks %q: %s", tc.contains, rec.Body)
			}
		})
	}
}

func TestEmptyListsAreArrays(t *testing.T) {
	h := newHandler()
	for _, path := range []string{"/api/patterns", "/api/leaderboard", "/api/battles"} {
		if got := strings.TrimSpace(do(h, "GET", path, "").Body.String()); got != "[]" {
			t.Errorf("%s = %s, want []", path, got)
		}
	}
}

func TestBattleRoutes(t *testing.T) {
	h := newHandler()
	cases := []struct {
		name, method, path, body string
		status                   int
		contains                 string
	}{
		{"create pattern a", "POST", "/api/patterns", goodBody, 201, `"id":"p_000001"`},
		{"create pattern b", "POST", "/api/patterns", goodBody, 201, `"id":"p_000002"`},
		{"create battle missing patternAId", "POST", "/api/battles", `{"patternBId":"p_000002"}`, 400, `"error"`},
		{"create battle same id", "POST", "/api/battles",
			`{"patternAId":"p_000001","patternBId":"p_000001"}`, 400, `"error"`},
		{"create battle unknown pattern id", "POST", "/api/battles",
			`{"patternAId":"p_zzzzzz","patternBId":"p_000002"}`, 404, `"error"`},
		{"create battle success", "POST", "/api/battles",
			`{"patternAId":"p_000001","patternBId":"p_000002"}`, 201,
			`"id":"b_000003","patternA":{"id":"p_000001"`},
		{"list battles", "GET", "/api/battles", "", 200, `"id":"b_000003"`},
		{"list battles limit", "GET", "/api/battles?limit=1", "", 200, `"id":"b_000003"`},
		{"get battle", "GET", "/api/battles/b_000003", "", 200, `"patternB":{"id":"p_000002"`},
		{"get battle unknown", "GET", "/api/battles/b_zzzzzz", "", 404, `"error"`},
		{"vote battle side a", "POST", "/api/battles/b_000003/votes", `{"side":"a"}`, 200, `"votesA":1,"votesB":0`},
		{"vote battle side b", "POST", "/api/battles/b_000003/votes", `{"side":"b"}`, 200, `"votesA":1,"votesB":1`},
		{"vote battle bad side", "POST", "/api/battles/b_000003/votes", `{"side":"x"}`, 400, `"error"`},
		{"vote battle missing side", "POST", "/api/battles/b_000003/votes", `{}`, 400, `"error"`},
		{"vote battle unknown field", "POST", "/api/battles/b_000003/votes", `{"side":"a","extra":1}`, 400, `"error"`},
		{"vote battle unknown battle", "POST", "/api/battles/b_zzzzzz/votes", `{"side":"a"}`, 404, `"error"`},
		{"leaderboard shows battle votes", "GET", "/api/leaderboard", "", 200, `"votes":1`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(h, tc.method, tc.path, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("%s %s = %d, want %d: %s", tc.method, tc.path, rec.Code, tc.status, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), tc.contains) {
				t.Errorf("body lacks %q: %s", tc.contains, rec.Body)
			}
		})
	}
}

func TestCreatedPatternShape(t *testing.T) {
	rec := do(newHandler(), "POST", "/api/patterns", goodBody)
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["createdAt"] != "2026-10-26T21:00:00Z" || got["votes"] != float64(0) || got["swing"] != float64(0) {
		t.Errorf("pattern = %v", got)
	}
}
