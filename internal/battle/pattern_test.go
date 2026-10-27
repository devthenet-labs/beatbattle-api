package battle

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
)

func steps(on ...int) []bool {
	s := make([]bool, Steps)
	for _, i := range on {
		s[i] = true
	}
	return s
}

func valid() Input {
	return Input{
		Name: "Four on the floor", Author: "brvtl", BPM: 120,
		Tracks: map[string][]bool{"kick": steps(0, 4, 8, 12), "snare": steps(4, 12), "hat": steps(0, 2)},
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Input)
		ok     bool
	}{
		{"valid", func(*Input) {}, true},
		{"name trimmed to 1", func(in *Input) { in.Name = "  x  " }, true},
		{"name 40 runes", func(in *Input) { in.Name = strings.Repeat("é", 40) }, true},
		{"name 41 runes", func(in *Input) { in.Name = strings.Repeat("é", 41) }, false},
		{"name blank", func(in *Input) { in.Name = "   " }, false},
		{"name control", func(in *Input) { in.Name = "a\tb" }, false},
		{"name newline", func(in *Input) { in.Name = "a\nb" }, false},
		{"author 24", func(in *Input) { in.Author = strings.Repeat("a", 24) }, true},
		{"author 25", func(in *Input) { in.Author = strings.Repeat("a", 25) }, false},
		{"author empty", func(in *Input) { in.Author = "" }, false},
		{"author charset", func(in *Input) { in.Author = "Ab_9-z" }, true},
		{"author space", func(in *Input) { in.Author = "a b" }, false},
		{"author unicode", func(in *Input) { in.Author = "bé" }, false},
		{"bpm 60", func(in *Input) { in.BPM = 60 }, true},
		{"bpm 200", func(in *Input) { in.BPM = 200 }, true},
		{"bpm 59", func(in *Input) { in.BPM = 59 }, false},
		{"bpm 201", func(in *Input) { in.BPM = 201 }, false},
		{"bpm missing", func(in *Input) { in.BPM = 0 }, false},
		{"swing 60", func(in *Input) { in.Swing = 60 }, true},
		{"swing 61", func(in *Input) { in.Swing = 61 }, false},
		{"swing -1", func(in *Input) { in.Swing = -1 }, false},
		{"track missing", func(in *Input) { delete(in.Tracks, "hat") }, false},
		{"track unknown", func(in *Input) { in.Tracks["clap"] = steps() }, false},
		{"track short", func(in *Input) { in.Tracks["kick"] = make([]bool, 15) }, false},
		{"track long", func(in *Input) { in.Tracks["kick"] = make([]bool, 17) }, false},
		{"tracks nil", func(in *Input) { in.Tracks = nil }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := valid()
			tc.mutate(&in)
			_, err := in.Validate()
			if (err == nil) != tc.ok {
				t.Fatalf("Validate() error = %v, want ok=%v", err, tc.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Errorf("error %v does not wrap ErrInvalid", err)
			}
		})
	}
}

func TestParseInput(t *testing.T) {
	cases := []struct {
		name, body string
		ok         bool
	}{
		{"minimal", `{"name":"a","author":"b","bpm":90,"tracks":{}}`, true},
		{"server fields ignored", `{"id":"p_x","votes":9,"createdAt":"2020-01-01T00:00:00Z","name":"a"}`, true},
		{"unknown field", `{"name":"a","extra":1}`, false},
		{"bpm fraction", `{"bpm":120.5}`, false},
		{"bpm string", `{"bpm":"120"}`, false},
		{"not json", `nope`, false},
		{"empty", ``, false},
		{"trailing data", `{"name":"a"} {"name":"b"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseInput(strings.NewReader(tc.body))
			if (err == nil) != tc.ok {
				t.Fatalf("ParseInput() error = %v, want ok=%v", err, tc.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Errorf("error %v does not wrap ErrInvalid", err)
			}
		})
	}
}

// TestValidateProperty checks, over seeded random inputs, that Validate agrees
// with an independent restatement of the contract.
func TestValidateProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(20261026))
	const authorAlphabet = "abcXYZ019_- é!"
	pick := func(alphabet []rune, n int) string {
		out := make([]rune, n)
		for i := range out {
			out[i] = alphabet[rng.Intn(len(alphabet))]
		}
		return string(out)
	}
	for i := 0; i < 2000; i++ {
		in := Input{
			Name:   pick([]rune("ab \t\u00e9"), rng.Intn(45)),
			Author: pick([]rune(authorAlphabet), rng.Intn(28)),
			BPM:    rng.Intn(260),
			Swing:  rng.Intn(80) - 5,
			Tracks: map[string][]bool{},
		}
		for _, n := range []string{"kick", "snare", "hat"} {
			if rng.Intn(10) > 0 {
				in.Tracks[n] = make([]bool, 14+rng.Intn(5))
			}
		}
		if rng.Intn(10) == 0 {
			in.Tracks["clap"] = make([]bool, Steps)
		}
		want := oracle(in)
		_, err := in.Validate()
		if (err == nil) != want {
			t.Fatalf("input %d %+v: Validate error = %v, oracle says valid=%v", i, in, err, want)
		}
	}
}

func oracle(in Input) bool {
	name := strings.TrimSpace(in.Name)
	runes := []rune(name)
	if len(runes) < 1 || len(runes) > 40 || strings.ContainsAny(name, "\t\n\r") {
		return false
	}
	if len(in.Author) < 1 || len(in.Author) > 24 {
		return false
	}
	for _, r := range in.Author {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-", r) {
			return false
		}
	}
	if in.BPM < 60 || in.BPM > 200 || in.Swing < 0 || in.Swing > 60 || len(in.Tracks) != 3 {
		return false
	}
	for _, n := range []string{"kick", "snare", "hat"} {
		if len(in.Tracks[n]) != Steps {
			return false
		}
	}
	return true
}
