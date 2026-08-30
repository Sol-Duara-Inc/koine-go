package wire

import (
	"errors"
	"strconv"

	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/codec"
)

// The guest side of the contract, written so that everything except the six
// import stubs and the three export bodies is ordinary Go that runs and is
// tested off-target. A wasm module is a hard place to debug; almost none of
// what follows needs to be debugged there — and the conformance module runs
// the rest of it inside the real sandbox, against the real loader.
//
// # Who a stoppage is attributed to
//
// Ruled 2026-08-28 (koine-go#12, second review): a breach is a VALID
// response — "a predefined direction to take for an unexpected stoppage on
// the tool side; Conduit is functioning properly" — and what matters is
// which party a stoppage is attributed to. Two kinds of stoppage reach this
// code, and they must not arrive at the record wearing each other's name:
//
//   - The AUTHOR's. A body that yields something no stratum generated has
//     written a program that cannot be run. It traps, and koinehost records
//     "trap in resolve:", which is the truth.
//   - BELOW THE LINE's. The host would not open an exchange, answered a
//     value poll with nothing, or handed back a frame this build cannot
//     read. None of that is the body's doing, and it used to trap too —
//     arriving at the record as the author's trap, for a condition that was
//     Conduit's or a tool's silence.
//
// The second kind no longer traps. It is recorded on the guest, spoken once
// through host_log with FaultPrefix in front of it, and gates every
// subsequent yield: nothing after a stoppage is stored, so a body cannot
// turn one into an event naming something that never happened. The resolve
// then ends Unanswered, and the status the host reads is a different number
// from anything an author can cause.
//
// It cannot be done by unwinding: TinyGo's wasm-unknown target traps on
// panic rather than running deferred recovers — verified, not assumed — so a
// guest in the sandbox has no way back from a panic. Cooperative it is.

// Host is the guest→host surface of §8, behind an interface so the guest
// runtime is testable without a sandbox. The wasm build wires it to the
// //go:wasmimport stubs; a test wires it to a scripted host. There is no
// third implementation and no room for one — the interface is the entire
// surface a guest can reach.
type Host interface {
	// Yield hands one utterance over. The host forms the envelope,
	// continues the chain, and stores by emitting. It reports the HOST'S
	// code: zero is success, and anything else is a host that could not
	// take it.
	Yield(frame []byte) uint32
	// Exchange utters an intent and answers with the handle the broker
	// minted, or zero for a refusal.
	Exchange(frame []byte) uint64
	// AckPoll asks the fast beat. Non-zero means the broker has it.
	AckPoll(handle uint64) uint32
	// ValuePoll waits until the exchange is filled or breached and
	// answers the response bytes.
	ValuePoll(handle uint64) []byte
	// Log carries a diagnostic line beside the run. It is not an emit
	// path: the host records it and never stores it as an event.
	Log(msg string)
}

// Speakable is what an utterance must be able to do to cross the boundary:
// name the event type it carries on the record, and write its own keys
// without reflection. Every generated stratum type can; nothing else can,
// which is the point.
//
// It is EncodeFields rather than MarshalJSON because the bytes the host
// stores are the domain object itself — the type key is written beside the
// object's keys, not around them — and splicing a key into finished JSON
// would mean string surgery on a payload this package does not own.
type Speakable interface {
	koine.Utterance
	EventType() string
	EncodeFields(*codec.Writer)
}

// Station is what one guest serves: one station, its derived manifest, and
// the one thing the wire cannot know — how to read this station's stratum
// out of the projected facts.
//
// Decode is written by the guest, not by this package, because the delivery
// type belongs to a stratum and the wire has no business knowing a stratum's
// shape. It is three lines: declare the generated delivery, unmarshal into
// it, hand it back.
type Station struct {
	// Koine is the station. It must be ADDRESSABLE — a pointer — because
	// the host fills its standard parts before Resolve, and standard
	// parts written into a copy are parts nobody can read.
	Koine koine.Koine
	// Manifest is the derived manifest JSON, exported under the name the
	// engine already reads (A2). It is embedded at build time, never
	// built here: a manifest this package could assemble is a manifest
	// that could disagree with the code.
	Manifest []byte
	// Decode reads the projected facts into this station's stratum.
	Decode func(DeliveryFrame) (koine.Delivery, error)
}

// FaultPrefix marks the one log line a guest speaks when it stops because
// the host could not answer it. It is a fixed string so that whoever reads
// Result.Logs — a person, or the engine — can attribute the stoppage without
// parsing prose.
const FaultPrefix = "koine/wire: stoppage below the line: "

// Guest is one wasm module's runtime: one station, one host below it, one
// arena. A module is a singleton by nature, and so is this.
type Guest struct {
	station Station
	host    Host
	arena   arena
	// refusal holds why the last delivery was refused, so a host reading
	// the outcome code can ask for the sentence behind it.
	refusal string
	// fault holds why the last delivery stopped for a reason below the
	// line. It gates every yield after it, so nothing is stored past a
	// stoppage the body did not cause and cannot fix.
	fault string
	// passing is the one-passage-per-delivery gate. It is koine's, not
	// this package's, because the bench enforces the same rule and a
	// second copy would be a second thing to drift.
	passing koine.Passing
}

// New builds the guest runtime over a given host. Tests use it; the engine
// does not, because inside the sandbox there is exactly one host and it is
// not a choice anybody makes.
func New(st Station, h Host) *Guest {
	return &Guest{station: st, host: h}
}

// Serve builds the guest runtime over the sandbox below it. This is what a
// real guest calls, once, at module scope.
func Serve(st Station) *Guest {
	g := New(st, nil)
	g.host = Sandbox(g)
	return g
}

// Refusal is the sentence behind the last Refused outcome, or empty. It is
// read for a log line, never branched on: a refusal is terminal.
func (g *Guest) Refusal() string { return g.refusal }

// Fault is the sentence behind the last Unanswered outcome, or empty: what
// the host could not do, in words, attributed below the line.
func (g *Guest) Fault() string { return g.fault }

// Deliver resolves one delivery and reports how it ended. It is the portable
// core of the resolve export, and it is where nothing is stored on refusal
// (A9): a frame that cannot be read runs no body at all.
func (g *Guest) Deliver(frame []byte) Outcome {
	g.refusal, g.fault = "", ""
	g.passing.Reset()
	f, err := DecodeDelivery(frame)
	if err != nil {
		return g.refuse(err.Error())
	}
	if g.station.Decode == nil {
		return g.refuse("this guest declares no way to read its own stratum")
	}
	delivered, err := g.station.Decode(f)
	if err != nil {
		return g.refuse("the projected facts did not read into this station's stratum: " + err.Error())
	}
	if delivered == nil {
		return g.refuse("this guest read the projected facts into nothing")
	}

	// Construction is delivery. The chain and the actor are minted below
	// the line and handed up; a station observes them and never invents
	// one. ProjectContext stays nil: its content is deliberately unruled
	// (§4), and this package surfaces that rather than inventing a shape.
	if !koine.Construct(g.station.Koine, koine.Standing{
		Chain:   koine.ChainRef(f.ChainID),
		Actor:   koine.ActorRef(f.Actor),
		Lineage: lineage{g: g},
	}) {
		return g.refuse("this guest serves something that embeds no stratum base, which is not a station")
	}

	if b, ok := delivered.(koine.Bindable); ok {
		delivered = b.Bind(broker{g: g})
	}

	cancelled := false
	g.station.Koine.Resolve(delivered, func(u koine.Utterance) bool {
		// A stoppage below the line closes the gate. Whatever the body
		// goes on to say is not stored, because it is speech about a
		// world the body was told about incorrectly.
		if cancelled || g.fault != "" {
			return false
		}
		// The host answers ZERO for success. Reading that backwards
		// would turn every stored emission into a cancellation without
		// ever raising an error, which is why the polarity is written
		// out here in words and pinned by a test.
		if g.host.Yield(g.yieldFrame(u)) != 0 {
			cancelled = true
			return false
		}
		return true
	})

	// The named hooks, run as SUGAR over the three verbs and never as a
	// second mechanism. The sequence is koine.RunHooks — one
	// implementation, shared with the harness, so the bench and the
	// sandbox cannot drift apart about what a hook means.
	if g.fault == "" {
		if err := koine.RunHooks(g.station.Koine, delivered, lineage{g: g}); err != nil {
			// A station that declared Post without Pre wrote a shape
			// that cannot run, which is the author's own mistake and
			// not a host that could not answer. It traps, and the
			// record names the right party.
			panic("koine/wire: " + err.Error())
		}
	}

	switch {
	case g.fault != "":
		return Unanswered
	case cancelled:
		g.host.Log(FaultPrefix + "the host would not take an utterance this station spoke; nothing after it was said")
		return Cancelled
	}
	return Resolved
}

// yieldFrame renders one utterance. A body that yields something no stratum
// generated TRAPS, and that trap is correctly the author's: it is a program
// that cannot be run, not a host that could not answer.
func (g *Guest) yieldFrame(u koine.Utterance) []byte {
	speakable, ok := u.(Speakable)
	if !ok {
		panic("koine/wire: a station yielded something no stratum generated — the emit path carries named domain objects, and there is no second channel to carry anything else")
	}
	frame, err := YieldFrame{Type: speakable.EventType(), Body: speakable}.MarshalJSON()
	if err != nil {
		panic("koine/wire: an utterance could not render itself: " + err.Error())
	}
	return frame
}

func (g *Guest) refuse(why string) Outcome {
	g.refusal = "koine/wire: " + why
	if g.host != nil {
		g.host.Log(FaultPrefix + why)
	}
	return Refused
}

// faultBelowTheLine records a stoppage the body did not cause, says it once
// through the host's own diagnostic channel with FaultPrefix in front of it,
// and leaves it standing for the rest of the delivery. The first one wins:
// everything after it is downstream of the first thing that went wrong.
func (g *Guest) faultBelowTheLine(why string) {
	if g.fault != "" {
		return
	}
	g.fault = why
	g.host.Log(FaultPrefix + why)
}

// Stopped is what a body is handed when the exchange it awaited stopped for
// a reason BELOW the line: the host would not open it, answered with
// nothing, or answered with something this build cannot read.
//
// It is not a Variant. A Variant is the expected future going the other way,
// which is a fact about the body's domain; this is a fact about the
// machinery under it. A body may branch on it like any error, and it costs
// nothing to do so — but whatever it speaks afterwards is NOT stored, and
// the run ends Unanswered rather than successfully. An author cannot turn a
// stoppage below the line into an event naming something that never
// happened, however they write the branch.
type Stopped struct {
	// Why is the stoppage, in words, already attributed.
	Why string
}

func (s *Stopped) Error() string { return FaultPrefix + s.Why }

// Variant is the typed outcome variant of an expected response: the future
// went the other way.
//
// A breach is a VALID RESPONSE, not a malfunction — ruled 2026-08-28: "a
// predefined direction to take for an unexpected stoppage on the tool side;
// Conduit is functioning properly." The tool stopped; the fabric did what it
// was built to do about it, and said so. It arrives as a Go error because an
// error is how Go spells "branch here", and "intentionally throwing an error
// is a programming technique… it is the intent that determines this" — not
// because anything under the station failed.
//
// conduit-go#200 has since landed and split the two: a breach sets the
// flag, and Conduit being unable to answer sets an error with the flag
// clear. The engine's own tests say so by name —
// TestValuePollUnknownHandleIsNotABreach, TestValuePollDeadlineIsNotABreach.
// So a Variant now means what it says, and everything that is not a finding
// about the work arrives as *Stopped instead. The named carrier for a tool
// breach on the engine side is koine.ErrToolBreach; this SDK follows that
// definition rather than minting a rival one.
type Variant struct {
	Name   string
	Status int
}

func (v *Variant) Error() string {
	if v.Name == "" {
		return "koine/wire: the exchange breached with status " + strconv.Itoa(v.Status)
	}
	return v.Name
}

// AckBroker is who the fast beat names.
//
// Wire v1 carries no comprehender: the host's ack_poll answers an integer
// and nothing else, so the only party this guest can honestly name is the
// one that did acknowledge — the deployment's broker. Naming the FULFILLER
// there is a v2 field, and both sides move together when it lands. Until
// then this is the true answer to a smaller question, rather than a
// plausible answer to the right one.
const AckBroker = koine.ActorRef("conduit:broker")

// broker turns koine.Broker's three beats into three host calls. It is the
// only implementation of Broker the SDK ships for a live host, and a station
// body can no more reach it than it can reach the host.
type broker struct{ g *Guest }

// Speak utters the intent and waits on nothing.
func (b broker) Speak(ex koine.Exchange) koine.Token {
	if b.g.fault != "" {
		return 0
	}
	frame, err := NewExchangeFrame(ex).MarshalJSON()
	if err != nil {
		panic("koine/wire: an exchange frame could not render itself: " + err.Error())
	}
	handle := b.g.host.Exchange(frame)
	if handle == 0 {
		// A seat that will not open is not a domain outcome and it is
		// not the body's doing: registration should have refused it by
		// name (§7.3), or the deployment wired no broker at all. Either
		// way the stoppage belongs below the line.
		b.g.faultBelowTheLine("the host would not open " + strconv.Quote(ex.Name) +
			" at seat " + strconv.Quote(ex.Seat))
		return 0
	}
	return koine.Token(handle)
}

// Received is the fast beat. A zero Ack is an honest not-yet.
func (b broker) Received(t koine.Token) koine.Ack {
	if b.g.fault != "" || t == 0 {
		return koine.Ack{}
	}
	if b.g.host.AckPoll(uint64(t)) == 0 {
		return koine.Ack{}
	}
	return koine.Ack{By: AckBroker}
}

// Await waits until the exchange is filled or breached (E-C, amended
// 2026-08-27). The wait itself is the HOST's: value_poll does not return
// until the exchange has gone one way or the other, so there is no poll loop
// here and no timeout invented here — a budget is the engine's, minted from
// the manifest's topology, and a guest that made up its own would be a guest
// arguing with the calculus.
//
// The wait CAN fail, and saying so here matters more than the tidier
// sentence that used to stand in its place. When the host cannot answer at
// all — no broker wired, a handle it does not know, bytes this build cannot
// read, or an answer that is neither filled nor breached — this returns a
// *Stopped and records the stoppage below the line. It does not trap: a
// trap would reach the record as "trap in resolve", which reads as the
// author's fault for something that was never theirs.
func (b broker) Await(t koine.Token) koine.Answer {
	if b.g.fault != "" {
		return koine.Answer{Err: &Stopped{Why: b.g.fault}}
	}
	if t == 0 {
		return b.stopped("the host answered no handle for an exchange this station spoke")
	}
	raw := b.g.host.ValuePoll(uint64(t))
	if len(raw) == 0 {
		return b.stopped("the host answered a value poll with nothing — a value that is neither filled nor breached is not an answer")
	}
	answer, err := DecodeAnswer(raw)
	if err != nil {
		return b.stopped("the host answered a value poll with a frame this build cannot read: " + err.Error())
	}
	// A breach is a valid response. The tool stopped; the fabric said so.
	if answer.Breach() {
		return koine.Answer{By: AckBroker, Err: &Variant{Name: answer.Error, Status: answer.Status}}
	}
	// The host said it could not answer, and did NOT claim a breach. Since
	// conduit-go#200 those are different sentences on the wire, so they are
	// different sentences here: this one is attributed below the line.
	if answer.Stoppage() {
		return b.stopped("the host answered status " + strconv.Itoa(answer.Status) + ": " + answer.Error)
	}
	// Filled, with nothing in it. Handing the body a zero value here is
	// how a fulfiller's silence became a stored event naming a deployment
	// that never happened; a fulfiller with nothing to say breaches.
	if len(answer.Value) == 0 {
		return b.stopped("the host answered status " + strconv.Itoa(answer.Status) +
			" with no value — a fulfiller with nothing to say breaches, it does not answer emptily")
	}
	return koine.Answer{By: AckBroker, JSON: answer.Value}
}

func (b broker) stopped(why string) koine.Answer {
	b.g.faultBelowTheLine(why)
	return koine.Answer{Err: &Stopped{Why: why}}
}

// ErrNoHost is what an off-target build of the host bindings answers with.
// It exists so that a native build of a guest fails loudly at the boundary
// rather than pretending to be in a sandbox.
var ErrNoHost = errors.New("koine/wire: there is no host below this build — a guest runs in the engine's sandbox, and nothing else is offered")
