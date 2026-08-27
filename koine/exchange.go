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
// way. Err is never transport — branch on it, don't defend against it.
type Answer struct {
	By   ActorRef
	JSON []byte
	Err  error
}

// Broker answers spoken exchanges. The engine implements it below the line;
// koine/testing implements it from a script. A station body never holds one
// and there is no way to reach one from inside Resolve — the verb is the
// only door.
//
// Consumed is the beat that says the caller materialized the answer — that
// this exchange was inline rather than a gate the station left standing. The
// host already knows this statically, from the manifest koinegen derived;
// hearing it again at run time is what lets a conformance test prove the
// declaration and the body agree (A3).
type Broker interface {
	Speak(Exchange) Answer
	Consumed(Exchange)
}

// Bindable is a generated delivery that can be wired to the broker standing
// below it. Construction is delivery: the host binds at construction, the
// harness binds from the script, and an unbound delivery's verbs panic
// rather than answer quietly. Authors write delivery literals; they never
// call Bind.
type Bindable interface {
	Bind(Broker) Delivery
}
