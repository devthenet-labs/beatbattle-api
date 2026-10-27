// Package battle is the pure core of beatbattle-api: pattern validation, the
// store and render-queue ports, and the leaderboard ordering. It has no
// net/http in it; time and randomness are injected.
package battle

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Steps is the length of every track.
const Steps = 16

// Track names, the only keys a pattern may carry.
var trackNames = []string{"kick", "snare", "hat"}

// Pattern is a stored 16-step drum pattern, in the shared JSON shape.
type Pattern struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Author    string            `json:"author"`
	BPM       int               `json:"bpm"`
	Swing     int               `json:"swing"`
	Tracks    map[string][]bool `json:"tracks"`
	Votes     int               `json:"votes"`
	Rating    int               `json:"rating"`
	CreatedAt time.Time         `json:"createdAt"`
}

// RatingEntry is the reduced shape served by GET /api/ratings.
type RatingEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Author string `json:"author"`
	Rating int    `json:"rating"`
}

// Clone returns a copy that shares no slices or maps with p.
func (p Pattern) Clone() Pattern {
	tracks := make(map[string][]bool, len(p.Tracks))
	for name, steps := range p.Tracks {
		tracks[name] = append([]bool(nil), steps...)
	}
	p.Tracks = tracks
	return p
}

// Input is what a client submits. The server-assigned fields are accepted and
// ignored, so a pattern fetched from the api can be posted back.
type Input struct {
	Name      string            `json:"name"`
	Author    string            `json:"author"`
	BPM       int               `json:"bpm"`
	Swing     int               `json:"swing"`
	Tracks    map[string][]bool `json:"tracks"`
	ID        json.RawMessage   `json:"id"`
	Votes     json.RawMessage   `json:"votes"`
	Rating    json.RawMessage   `json:"rating"`
	CreatedAt json.RawMessage   `json:"createdAt"`
}

// ErrInvalid marks every validation failure; the message says what is wrong.
var ErrInvalid = errors.New("invalid pattern")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// ParseInput decodes one pattern from r, rejecting unknown fields and any
// trailing data. It does not validate the values; see Input.Validate.
func ParseInput(r io.Reader) (Input, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var in Input
	if err := dec.Decode(&in); err != nil {
		return Input{}, invalid("malformed JSON: %v", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Input{}, invalid("unexpected data after the JSON object")
	}
	return in, nil
}

// Validate checks every rule of the shared pattern contract and returns the
// normalised Input (trimmed name).
func (in Input) Validate() (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 40 {
		return in, invalid("name must be 1-40 characters")
	}
	if strings.ContainsFunc(in.Name, unicode.IsControl) {
		return in, invalid("name must not contain control characters")
	}
	if n := len(in.Author); n < 1 || n > 24 || !validAuthor(in.Author) {
		return in, invalid("author must be 1-24 characters from A-Z a-z 0-9 _ -")
	}
	if in.BPM < 60 || in.BPM > 200 {
		return in, invalid("bpm must be an integer from 60 to 200")
	}
	if in.Swing < 0 || in.Swing > 60 {
		return in, invalid("swing must be an integer from 0 to 60")
	}
	return in, validTracks(in.Tracks)
}

func validAuthor(s string) bool {
	for _, r := range s {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-'
		if !ok {
			return false
		}
	}
	return true
}

func validTracks(tracks map[string][]bool) error {
	for name := range tracks {
		if !knownTrack(name) {
			return invalid("unknown track %q", name)
		}
	}
	for _, name := range trackNames {
		steps, ok := tracks[name]
		if !ok {
			return invalid("track %q is missing", name)
		}
		if len(steps) != Steps {
			return invalid("track %q must have exactly %d steps", name, Steps)
		}
	}
	return nil
}

func knownTrack(name string) bool {
	for _, n := range trackNames {
		if n == name {
			return true
		}
	}
	return false
}
