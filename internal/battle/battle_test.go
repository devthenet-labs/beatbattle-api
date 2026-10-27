package battle

import (
	"errors"
	"strings"
	"testing"
)

func TestBattleInputValidate(t *testing.T) {
	cases := []struct {
		name string
		in   BattleInput
		ok   bool
	}{
		{"valid", BattleInput{PatternAID: "p_1", PatternBID: "p_2"}, true},
		{"patternAId missing", BattleInput{PatternBID: "p_2"}, false},
		{"patternBId missing", BattleInput{PatternAID: "p_1"}, false},
		{"both missing", BattleInput{}, false},
		{"same id", BattleInput{PatternAID: "p_1", PatternBID: "p_1"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.in.Validate()
			if (err == nil) != tc.ok {
				t.Fatalf("Validate() error = %v, want ok=%v", err, tc.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Errorf("error %v does not wrap ErrInvalid", err)
			}
		})
	}
}

func TestParseBattleInput(t *testing.T) {
	cases := []struct {
		name, body string
		ok         bool
	}{
		{"minimal", `{"patternAId":"p_1","patternBId":"p_2"}`, true},
		{"unknown field", `{"patternAId":"p_1","patternBId":"p_2","extra":1}`, false},
		{"not json", `nope`, false},
		{"empty", ``, false},
		{"trailing data", `{"patternAId":"p_1"} {"patternBId":"p_2"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseBattleInput(strings.NewReader(tc.body))
			if (err == nil) != tc.ok {
				t.Fatalf("ParseBattleInput() error = %v, want ok=%v", err, tc.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Errorf("error %v does not wrap ErrInvalid", err)
			}
		})
	}
}

func TestBattleVoteInputValidate(t *testing.T) {
	cases := []struct {
		name string
		in   BattleVoteInput
		ok   bool
	}{
		{"a", BattleVoteInput{Side: "a"}, true},
		{"b", BattleVoteInput{Side: "b"}, true},
		{"empty", BattleVoteInput{}, false},
		{"upper", BattleVoteInput{Side: "A"}, false},
		{"other", BattleVoteInput{Side: "c"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			side, err := tc.in.Validate()
			if (err == nil) != tc.ok {
				t.Fatalf("Validate() error = %v, want ok=%v", err, tc.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Errorf("error %v does not wrap ErrInvalid", err)
			}
			if err == nil && side != tc.in.Side {
				t.Errorf("side = %q, want %q", side, tc.in.Side)
			}
		})
	}
}

func TestParseBattleVoteInput(t *testing.T) {
	cases := []struct {
		name, body string
		ok         bool
	}{
		{"minimal", `{"side":"a"}`, true},
		{"unknown field", `{"side":"a","extra":1}`, false},
		{"not json", `nope`, false},
		{"empty", ``, false},
		{"trailing data", `{"side":"a"} {"side":"b"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseBattleVoteInput(strings.NewReader(tc.body))
			if (err == nil) != tc.ok {
				t.Fatalf("ParseBattleVoteInput() error = %v, want ok=%v", err, tc.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Errorf("error %v does not wrap ErrInvalid", err)
			}
		})
	}
}
