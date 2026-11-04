// Package battle is the pure core of beatbattle-api: pattern validation, the
// store and render-queue ports, and the leaderboard ordering. It has no
// net/http in it; time and randomness are injected.
package battle

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Steps is the length of every track.
const Steps = 16

// Track names, the only keys a pattern may carry.
var trackNames = []string{"kick", "snare", "hat"}

// Pattern is a stored 16-step drum pattern, in the shared JSON shape. The
// field order of ID, ParentID, RemixCount, Name matters: it fixes the served
// JSON key order that the list acceptance checks depend on. Do not reorder.
// Level is a derived method, not a field; MarshalJSON appends it as "level"
// after all of these fields on serialization.
type Pattern struct {
	ID          string            `json:"id"`
	ParentID    string            `json:"parentId"`
	RemixCount  int               `json:"remixCount"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	BPM         int               `json:"bpm"`
	Swing       int               `json:"swing"`
	LengthMs    int               `json:"lengthMs"`
	Tracks      map[string][]bool `json:"tracks"`
	Votes       int               `json:"votes"`
	Rating      int               `json:"rating"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// Level derives a coarse density label from the on-steps across all of p's
// tracks (48 cells total): "sparse" below 8, "busy" above 24, "groove"
// otherwise.
func (p Pattern) Level() string {
	on := 0
	for _, name := range trackNames {
		for _, s := range p.Tracks[name] {
			if s {
				on++
			}
		}
	}
	switch {
	case on < 8:
		return "sparse"
	case on > 24:
		return "busy"
	default:
		return "groove"
	}
}

// MarshalJSON serializes p's existing fields unchanged, then appends
// "level" after them.
func (p Pattern) MarshalJSON() ([]byte, error) {
	type patternAlias Pattern
	return json.Marshal(struct {
		patternAlias
		Level string `json:"level"`
	}{patternAlias(p), p.Level()})
}

// LengthMs returns how long one pass of Steps sixteenth notes lasts at
// bpm, in whole milliseconds, rounded to the nearest millisecond. Swing
// does not change it.
func LengthMs(bpm int) int {
	return int(math.Round(60000.0 / float64(bpm) * 4))
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
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	BPM         int               `json:"bpm"`
	Swing       int               `json:"swing"`
	Tracks      map[string][]bool `json:"tracks"`
	ID          json.RawMessage   `json:"id"`
	ParentID    json.RawMessage   `json:"parentId"`
	RemixCount  json.RawMessage   `json:"remixCount"`
	Votes       json.RawMessage   `json:"votes"`
	Rating      json.RawMessage   `json:"rating"`
	CreatedAt   json.RawMessage   `json:"createdAt"`
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
	name, err := validateName(in.Name)
	if err != nil {
		return in, err
	}
	in.Name = name
	description, err := validateDescription(in.Description)
	if err != nil {
		return in, err
	}
	in.Description = description
	if err := validateAuthorField(in.Author); err != nil {
		return in, err
	}
	if in.BPM < 60 || in.BPM > 200 {
		return in, invalid("bpm must be an integer from 60 to 200")
	}
	if in.Swing < 0 || in.Swing > 60 {
		return in, invalid("swing must be an integer from 0 to 60")
	}
	return in, validTracks(in.Tracks)
}

// validateName trims name and checks its length and character rules,
// returning the trimmed name.
func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 40 {
		return name, invalid("name must be 1-40 characters")
	}
	if strings.ContainsFunc(name, unicode.IsControl) {
		return name, invalid("name must not contain control characters")
	}
	return name, nil
}

// validateDescription trims d and checks its length and character rules,
// returning the trimmed description. An empty trimmed value is valid, since
// description is optional.
func validateDescription(d string) (string, error) {
	d = strings.TrimSpace(d)
	if utf8.RuneCountInString(d) > 140 {
		return d, invalid("description must be at most 140 characters")
	}
	if strings.ContainsFunc(d, unicode.IsControl) {
		return d, invalid("description must not contain control characters")
	}
	return d, nil
}

// validateAuthorField checks author's length and character rules.
func validateAuthorField(author string) error {
	if n := len(author); n < 1 || n > 24 || !validAuthor(author) {
		return invalid("author must be 1-24 characters from A-Z a-z 0-9 _ -")
	}
	return nil
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

// RemixInput is what a client submits to remix an existing pattern: a new
// name and author. The copied tracks/bpm/swing come from the original, not
// from the client.
type RemixInput struct {
	Name   string `json:"name"`
	Author string `json:"author"`
}

// ParseRemixInput decodes one remix input from r, rejecting unknown fields
// and any trailing data. It does not validate the values; see
// RemixInput.Validate.
func ParseRemixInput(r io.Reader) (RemixInput, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var in RemixInput
	if err := dec.Decode(&in); err != nil {
		return RemixInput{}, invalid("malformed JSON: %v", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return RemixInput{}, invalid("unexpected data after the JSON object")
	}
	return in, nil
}

// Validate checks the name and author rules and returns the normalised
// RemixInput (trimmed name).
func (in RemixInput) Validate() (RemixInput, error) {
	name, err := validateName(in.Name)
	if err != nil {
		return in, err
	}
	in.Name = name
	if err := validateAuthorField(in.Author); err != nil {
		return in, err
	}
	return in, nil
}
