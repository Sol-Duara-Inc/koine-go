package koine

// Handle is a spoken exchange, not yet consumed. How you use it IS the
// branch control (ratified 2026-07-30; restated as two cases, not three,
// 2026-08-27 — DESIGN.md §6, issue #11):
//
//   - Value() consumed        → inline: the exchange runs in your sequence.
//   - spoken, never consumed  → concurrent (default): your completion gates
//     on its resolution — nothing is silently ungated.
//
// From a station author's side there were only ever the two cases above: I
// waited for this value, or I did not. A third word once lived here,
// declaring that a workflow has another chainId to watch — a topology fact
// that belongs to the workflow author, writing a workflow YAML or a CDrus
// Expression, never to the station author writing Resolve. See DESIGN.md §6
// for the ruling and why the word was struck (2026-08-27, issue #11).
//
// Value waits until the exchange is filled or breached — ruled 2026-08-26:
// "the system must block on values that have not arrived." This is a
// programmer's platform; asynchrony is yours to program, and the platform is
// honest about it rather than protective. The error is the typed outcome
// variant of the expected response — never transport. Branch, don't defend.
type Handle[T any] interface {
	// Received gates on the fast beat: someone who declared comprehension
	// has this now.
	Received() Ack
	// Value gates on completion and materializes the answer.
	Value() (T, error)
}
