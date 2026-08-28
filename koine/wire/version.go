package wire

import "errors"

// Version is the contract this build speaks. It is a CONSTANT, not a
// negotiation: both sides gate on it, and a mismatch is a named refusal at
// load rather than a surprise in the middle of a resolve.
//
// The number moves when the shape of a frame changes — a key added, removed,
// renamed, or given a new meaning. It does not move when a station's
// delivery grows keys of its own: a stratum's shape is the stratum's, and
// the wire carries whatever it is handed (issue #4, owner: "carry whatever
// the shape is when you marshal it").
const Version = "koine.wire/1"

// ErrVersion is the refusal every frame reader raises on a foreign version.
// It is a sentinel so the engine's loader can name it in its own refusal
// ladder without matching on a string.
var ErrVersion = errors.New("koine/wire: frame speaks a version this build does not")

// Accepts judges one frame's declared version. It refuses by name, quoting
// both versions, because a version refusal is the one error a person reads
// at the exact moment they are least able to guess what happened.
func Accepts(declared string) error {
	if declared == Version {
		return nil
	}
	return &VersionError{Declared: declared, Speaks: Version}
}

// VersionError names both sides of a version refusal.
type VersionError struct {
	Declared string // what the frame said
	Speaks   string // what this build speaks
}

func (e *VersionError) Error() string {
	declared := e.Declared
	if declared == "" {
		declared = "nothing"
	}
	return "koine/wire: frame declares " + declared + "; this build speaks " + e.Speaks
}

// Unwrap lets errors.Is reach ErrVersion.
func (e *VersionError) Unwrap() error { return ErrVersion }
