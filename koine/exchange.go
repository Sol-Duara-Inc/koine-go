package koine

// This file is the host seam for §6's spoken exchanges. It adds nothing to
// §4's contract types and changes none of them: Handle, Ack and Outcome are
// exactly as K0 ratified them. What lands here is the shape a
// generated verb speaks through and the shape whoever stands below the
// station answers with — the host in a deployment, koine/testing under a
// test. A station body holds none of these types; it holds a Handle.

// Arg is one named argument of a spoken exchange, already rendered to its
// wire spelling by generated code. Rendering happens at the call site, in
// source, so nothing on this path reflects and the order is the order the
// author wrote.
type Arg struct {
	Name  string
	Value string
}

// Exchange is the request half of a spoken exchange: the seat asked, the
// intent spoken, and the arguments carried. There is one calling convention
// in the entire system and this is its request; no second channel exists
// anywhere.
type Exchange struct {
	Seat string // the seat that answers — "history"
	Name string // the intent spoken — "history.last"
	Args []Arg
}

// Answer is the response half: the acknowledging comprehender, the payload
// already projected to the caller's lineage, and the typed outcome variant
// that stands in the payload's place when the expected future went the other
// way. Branch on Err; don't defend against it.
//
// The design's stronger sentence — "Err is NEVER transport" — stays
// RETRACTED, and conduit-go#200 did not restore it. #200 split the wire's two
// answers, which is what lets koine/wire tell a finding about the work from
// Conduit being unable to answer; but BOTH still arrive here, in this one
// field. A *wire.Variant is the expected future going the other way, and a
// *wire.Stopped is the machinery underneath — and a body that treats them
// alike is a body that will one day store an event about a deadline.
//
// Branch on the kind, not just on non-nil. Splitting them into separate
// fields is wire v2, and both sides move together when it lands.
//
// By is subject to the same honesty: wire v1's host names no comprehender on
// either beat, so what arrives is the party that answered rather than the
// fulfiller that comprehended. See koine/wire.AckBroker.
type Answer struct {
	By   ActorRef
	JSON []byte
	Err  error
}

// Token names one exchange the host opened. The host mints it, the guest
// carries it, and it is meaningless anywhere else — a station body never
// sees one, because a station body holds a Handle.
type Token uint64

// Broker answers spoken exchanges. The engine implements it below the line
// over koine/wire; koine/testing implements it from a script. A station body
// never holds one and there is no way to reach one from inside Resolve — the
// verb is the only door.
//
// The three methods are the three beats of the work plane, and they are
// separate because §6's two consumption patterns require it. Speak utters
// the intent and waits on nothing; a station that speaks and walks away has
// gated its completion on the answer without ever asking for it. Await is
// the gate Value() stands at, waiting until the exchange is filled or
// breached (E-C, amended 2026-08-27).
//
// Being asked to Await IS the consumption beat: a station that awaits ran
// the exchange in its own sequence. The host already knows this statically,
// from the manifest koinegen derived; hearing it again at run time is what
// lets the conformance gate prove the declaration and the body agree (A3).
type Broker interface {
	// Speak utters the intent. Nothing is waited on.
	Speak(Exchange) Token
	// Received is the fast beat. A zero Ack means nobody has declared
	// comprehension yet — an honest "not yet", never an error.
	Received(Token) Ack
	// Await waits until the exchange is filled or breached, and is the
	// beat that says this exchange was consumed.
	Await(Token) Answer
}

// Bindable is a generated delivery that can be wired to the broker standing
// below it. Construction is delivery: the host binds at construction, the
// harness binds from the script, and an unbound delivery's verbs panic
// rather than answer quietly. Authors write delivery literals; they never
// call Bind.
type Bindable interface {
	Bind(Broker) Delivery
}
