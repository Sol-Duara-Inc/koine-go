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

// Detachable is the seam a generated handle fills so that a written Detach
// is also a spoken one. Nothing in Handle[T] changed to make room for it:
// Detach asks, and a handle that has nothing to tell says nothing.
type Detachable interface {
	// MarkDetached tells the host below this handle that the exchange was
	// released from the station's completion gate.
	MarkDetached()
}

// Detach releases an exchange from your completion gate — the same word as
// the grammar's keyword, one vocabulary across both layers. In code, under
// your name, in the blame: divergence from the default gate is legal and
// loud.
//
// It is read twice, and the two readings must agree. koinegen reads it from
// your source, which is what puts the detached chain role in the manifest
// before anything runs; and the call also tells the host below the handle,
// so a pure-Resolve run can WITNESS the release instead of inferring it.
// Ruled 2026-08-27, closing the third chain role: a declaration the
// conformance gate cannot see is a declaration the gate cannot pin, and A3
// is an absolute or it is nothing.
//
// A handle that is not a generated one — a hand-written double in a test —
// has no host to tell, and Detach stays the no-op it always was.
func Detach[T any](h Handle[T]) {
	if d, ok := any(h).(Detachable); ok {
		d.MarkDetached()
	}
}
