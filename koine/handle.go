package koine

// Handle is a spoken exchange, not yet consumed. How you use it IS the
// branch control (ratified 2026-07-30):
//
//   - Value() consumed        → inline: the exchange runs in your sequence.
//   - spoken, never consumed  → concurrent (default): your completion gates
//     on its resolution — nothing is silently ungated.
//   - Detach(h)               → detached, by declaration, under your name.
//
// Value blocks until the answer arrives — ruled 2026-08-26: "the system must
// block on values that have not arrived." This is a programmer's platform;
// asynchrony is yours to program, and the platform is honest about it rather
// than protective. The error is the typed outcome variant of the expected
// response — never transport. Branch, don't defend.
type Handle[T any] interface {
	// Received gates on the fast beat: someone who declared comprehension
	// has this now.
	Received() Ack
	// Value gates on completion and materializes the answer.
	Value() (T, error)
}

// Detach releases an exchange from your completion gate — the same word as
// the grammar's keyword, one vocabulary across both layers. It is a
// declaration, not an action: the analyzer reads it from your code, and the
// host honors it. In code, under your name, in the blame — divergence from
// the default gate is legal and loud.
func Detach[T any](Handle[T]) {}
