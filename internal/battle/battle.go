package battle

import (
	"encoding/json"
	"errors"
	"io"
	"time"
)

// BattleInput is what a client submits to start a battle.
type BattleInput struct {
	PatternAID string `json:"patternAId"`
	PatternBID string `json:"patternBId"`
}

// ParseBattleInput decodes one battle input from r, rejecting unknown fields
// and any trailing data. It does not validate the values; see
// BattleInput.Validate.
func ParseBattleInput(r io.Reader) (BattleInput, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var in BattleInput
	if err := dec.Decode(&in); err != nil {
		return BattleInput{}, invalid("malformed JSON: %v", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return BattleInput{}, invalid("unexpected data after the JSON object")
	}
	return in, nil
}

// Validate checks that both pattern ids are present and different. It does
// not look them up; the service does that against the pattern store.
func (in BattleInput) Validate() (BattleInput, error) {
	if in.PatternAID == "" {
		return in, invalid("patternAId is required")
	}
	if in.PatternBID == "" {
		return in, invalid("patternBId is required")
	}
	if in.PatternAID == in.PatternBID {
		return in, invalid("patternAId and patternBId must be different")
	}
	return in, nil
}

// BattleVoteInput is what a client submits to vote on a battle side.
type BattleVoteInput struct {
	Side string `json:"side"`
}

// ParseBattleVoteInput decodes one battle vote input from r, rejecting
// unknown fields and any trailing data. It does not validate the value; see
// BattleVoteInput.Validate.
func ParseBattleVoteInput(r io.Reader) (BattleVoteInput, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var in BattleVoteInput
	if err := dec.Decode(&in); err != nil {
		return BattleVoteInput{}, invalid("malformed JSON: %v", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return BattleVoteInput{}, invalid("unexpected data after the JSON object")
	}
	return in, nil
}

// Validate returns the side when it is exactly "a" or "b".
func (in BattleVoteInput) Validate() (string, error) {
	if in.Side != "a" && in.Side != "b" {
		return "", invalid(`side must be "a" or "b"`)
	}
	return in.Side, nil
}

// BattleStakes is the rating points each side of a battle would gain if
// it got the next vote, from the ratings as they stand now.
type BattleStakes struct {
	A int `json:"a"`
	B int `json:"b"`
}

// BattleView is the full shape served by every battle endpoint: a battle
// joined with its two live patterns.
type BattleView struct {
	ID        string       `json:"id"`
	PatternA  Pattern      `json:"patternA"`
	PatternB  Pattern      `json:"patternB"`
	VotesA    int          `json:"votesA"`
	VotesB    int          `json:"votesB"`
	CreatedAt time.Time    `json:"createdAt"`
	Stakes    BattleStakes `json:"stakes"`
}
