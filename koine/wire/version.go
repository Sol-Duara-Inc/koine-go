package wire

import (
	"errors"
	"strconv"
)

// Version is the contract this build speaks. It is koinehost.WireVersion —
// an integer, because that is what the host's Delivery.Version field is, and
// the host is normative (ruled 2026-08-28). It is a CONSTANT, not a
// negotiation: both sides gate on it, and a mismatch is a named refusal
// rather than a surprise in the middle of a resolve.
//
// The number moves when the shape of a frame changes — a key added, removed,
// renamed, or given a new meaning — AND BOTH SIDES MOVE TOGETHER. That is
// the discipline this contract's first attempt lacked: two halves shipped
// against two readings of one wire, each green in its own repository. The
// conformance module in this repository is what makes moving apart fail.
const Version = 1

// ErrVersion is the refusal every frame reader raises on a foreign version.
// It is a sentinel so the engine's loader can name it in its own refusal
// ladder without matching on a string.
var ErrVersion = errors.New("koine/wire: frame speaks a version this build does not")

// Accepts judges one frame's declared version. It refuses by name, quoting
// both versions, because a version refusal is the one error a person reads
// at the exact moment they are least able to guess what happened.
func Accepts(declared int) error {
	if declared == Version {
		return nil
	}
	return &VersionError{Declared: declared, Speaks: Version}
}

// VersionError names both sides of a version refusal.
type VersionError struct {
	Declared int // what the frame said
	Speaks   int // what this build speaks
}

func (e *VersionError) Error() string {
	declared := strconv.Itoa(e.Declared)
	if e.Declared == 0 {
		declared = "no version at all"
	}
	return "koine/wire: frame declares " + declared + "; this build speaks " + strconv.Itoa(e.Speaks)
}

// Unwrap lets errors.Is reach ErrVersion.
func (e *VersionError) Unwrap() error { return ErrVersion }
