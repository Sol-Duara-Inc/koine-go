// Package selector is the awaits grammar: the shapes a station declares it
// knows what to do with. A Selector is DATA, not behavior — the host matches;
// this package describes. The described form is what rides the manifest, so
// the shape is stable, documented, and serializes without reflection tricks:
// every field is exported and tagged.
//
// The resolved idiom (ratified 2026-07-20) is structural here: a resolved
// selector awaits the COMPLETED thought — the subject's terminal set, either
// outcome — and the grammar deliberately has no outcome filter to write.
// There is no Succeeded() and no Failed(); the station's body branches on
// outcome, because code is written against BOTH branches of the expected
// future.
package selector

// Mode says which kind of expectation a Selector declares.
type Mode string

const (
	// ModeEvent awaits one arrival of the named shape.
	ModeEvent Mode = "event"
	// ModeResolved awaits the completed thought: the subject's terminal
	// set, either outcome.
	ModeResolved Mode = "resolved"
	// ModeAbsent awaits the expectation that did not arrive — absence,
	// declared as a shape like any other.
	ModeAbsent Mode = "absent"
)

// Selector declares one awaited shape. The zero value is not a valid
// selector; use the constructors.
type Selector struct {
	// Type names the awaited shape: an event type for ModeEvent/ModeAbsent,
	// a subject for ModeResolved.
	Type string `json:"type"`
	// Anchor optionally binds the arrival to an anchor name, so the body
	// and the workflow speak about the same position with the same word.
	Anchor string `json:"anchor,omitempty"`
	// Mode is the kind of expectation.
	Mode Mode `json:"mode"`
}

// Event awaits one arrival of the named event type.
func Event(eventType string) Selector { return Selector{Type: eventType, Mode: ModeEvent} }

// Resolved awaits the subject's completed thought — its terminal set, either
// outcome. Deliberately outcome-blind: branch in the body, never in the
// declaration.
func Resolved(subject string) Selector { return Selector{Type: subject, Mode: ModeResolved} }

// Absent declares the absence of the given expectation as itself an awaited
// shape. Value semantics: the argument is unchanged.
func Absent(s Selector) Selector { s.Mode = ModeAbsent; return s }

// At binds the selector to an anchor name. Value semantics.
func (s Selector) At(anchor string) Selector { s.Anchor = anchor; return s }

// List is the declaration form Awaits() returns. It copies, so a station's
// declaration cannot be mutated through a retained argument slice.
func List(ss ...Selector) []Selector { return append([]Selector(nil), ss...) }
