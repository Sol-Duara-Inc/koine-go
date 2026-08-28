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

// Guest is one wasm module's runtime: one station, one host below it, one
// arena. A module is a singleton by nature, and so is this.
type Guest struct {
	station Station
	host    Host
	arena   arena
	// refusal holds why the last delivery was refused, so a host reading
	// the outcome code can ask for the sentence behind it.
	refusal string
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

// Deliver resolves one delivery and reports how it ended. It is the portable
// core of the resolve export, and it is where nothing is stored on refusal
// (A9): a frame that cannot be read runs no body at all.
func (g *Guest) Deliver(frame []byte) Outcome {
	g.refusal = ""
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
	if !koine.Construct(g.station.Koine, koine.ChainRef(f.ChainID), koine.ActorRef(f.Actor), nil) {
		return g.refuse("this guest serves something that embeds no stratum base, which is not a station")
	}

	if b, ok := delivered.(koine.Bindable); ok {
		delivered = b.Bind(broker{host: g.host})
	}

	cancelled := false
	g.station.Koine.Resolve(delivered, func(u koine.Utterance) bool {
		if cancelled {
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
	if cancelled {
		return Cancelled
	}
	return Resolved
}

// yieldFrame renders one utterance. A body that yields something no stratum
// generated traps rather than sending a shape the record cannot store: the
// guest has one emit path, and it carries named domain objects or nothing.
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
	return Refused
}

// Variant is the typed outcome variant of an expected response: the future
// went the other way. It is never transport — there is no transport in this
// vocabulary to report — so a station branches on it rather than defending
// against it.
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
// there is a v2 field, and both sides move together when it lands (see the
// PR that ruled the host normative). Until then this is the true answer to
// a smaller question, rather than a plausible answer to the right one.
const AckBroker = koine.ActorRef("conduit:broker")

// broker turns koine.Broker's three beats into three host calls. It is the
// only implementation of Broker the SDK ships for a live host, and a station
// body can no more reach it than it can reach the host.
type broker struct{ host Host }

// Speak utters the intent and waits on nothing.
func (b broker) Speak(ex koine.Exchange) koine.Token {
	frame, err := NewExchangeFrame(ex).MarshalJSON()
	if err != nil {
		panic("koine/wire: an exchange frame could not render itself: " + err.Error())
	}
	handle := b.host.Exchange(frame)
	if handle == 0 {
		// A seat that will not open is not a domain outcome; it is a
		// deployment that registration should already have refused by
		// name (§7.3). Trapping is the honest end — the host attributes
		// it as work.finished{outcome: failure}.
		panic("koine/wire: the host would not open " + strconv.Quote(ex.Name) + " at seat " + strconv.Quote(ex.Seat) + " — an exchange never fails silently")
	}
	return koine.Token(handle)
}

// Received is the fast beat. A zero Ack is an honest not-yet.
func (b broker) Received(t koine.Token) koine.Ack {
	if b.host.AckPoll(uint64(t)) == 0 {
		return koine.Ack{}
	}
	return koine.Ack{By: AckBroker}
}

// Await waits until the exchange is filled or breached (E-C, amended
// 2026-08-27). The wait itself is the HOST's: value_poll does not return
// until the exchange has gone one way or the other, which is exactly where
// the ruling left the mechanism. There is no poll loop here and no timeout
// invented here — a budget is the engine's, minted from the manifest's
// topology, and a guest that made up its own would be a guest arguing with
// the calculus.
func (b broker) Await(t koine.Token) koine.Answer {
	raw := b.host.ValuePoll(uint64(t))
	if len(raw) == 0 {
		panic("koine/wire: the host answered a value poll with nothing — a value that is neither filled nor breached is not an answer")
	}
	answer, err := DecodeAnswer(raw)
	if err != nil {
		panic("koine/wire: the host answered a value poll with a frame this build cannot read: " + err.Error())
	}
	if answer.Breach() {
		return koine.Answer{By: AckBroker, Err: &Variant{Name: answer.Error, Status: answer.Status}}
	}
	return koine.Answer{By: AckBroker, JSON: answer.Value}
}

// ErrNoHost is what an off-target build of the host bindings answers with.
// It exists so that a native build of a guest fails loudly at the boundary
// rather than pretending to be in a sandbox.
var ErrNoHost = errors.New("koine/wire: there is no host below this build — a guest runs in the engine's sandbox, and nothing else is offered")
