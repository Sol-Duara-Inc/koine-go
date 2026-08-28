package wire

import (
	"errors"
	"strconv"

	"github.com/sol-duara-inc/koine-go/koine"
)

// The guest side of the contract, written so that everything except the four
// import stubs is ordinary Go that runs and is tested off-target. A wasm
// module is a hard place to debug; almost none of what follows needs to be
// debugged there.

// Host is the four guest→host calls of §8, behind an interface so the guest
// runtime is testable without a sandbox. The wasm build wires it to the
// //go:wasmimport stubs; a test wires it to a scripted host. There is no
// third implementation and no room for one — the interface is the entire
// surface a guest can reach.
type Host interface {
	// Yield hands one utterance frame over. The host forms the envelope,
	// continues the chain, and stores by emitting. False is the host
	// cancelling: nothing after the refusal is spoken.
	Yield(frame []byte) bool
	// Exchange utters an intent and answers with an opened frame naming
	// the token, or naming a refusal.
	Exchange(frame []byte) []byte
	// AckPoll asks the fast beat.
	AckPoll(token uint64) []byte
	// ValuePoll asks whether the exchange is filled, breached, or still
	// pending.
	ValuePoll(token uint64) []byte
}

// Speakable is what an utterance must be able to do to cross the boundary:
// name the event type it carries on the record, and render itself without
// reflection. Every generated stratum type can; nothing else can, which is
// the point.
type Speakable interface {
	koine.Utterance
	EventType() string
	MarshalJSON() ([]byte, error)
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
// inbox. A module is a singleton by nature, and so is this.
type Guest struct {
	station Station
	host    Host
	inbox   []byte
	// refusal holds why the last delivery was refused, so a host reading
	// the outcome code can ask for the sentence behind it.
	refusal string
}

// New builds the guest runtime over a given host. Tests use it; the engine
// does not, because inside the sandbox there is exactly one host and it is
// not a choice anybody makes.
func New(st Station, h Host) *Guest {
	return &Guest{station: st, host: h, inbox: make([]byte, InboxCapacity)}
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
// core of the deliver export, and it is where nothing is stored on refusal
// (A9): a frame that cannot be read runs no body at all.
func (g *Guest) Deliver(frame []byte) Outcome {
	g.refusal = ""
	f, err := DecodeDelivery(frame)
	if err != nil {
		return g.refuse(err.Error())
	}
	claim := g.station.Koine.Identity()
	if f.Station != "" && f.Station != claim.Name {
		return g.refuse("frame addresses station " + strconv.Quote(f.Station) + "; this guest serves " + strconv.Quote(claim.Name))
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
	if !koine.Construct(g.station.Koine, koine.ChainRef(f.Chain), koine.ActorRef(f.Actor), nil) {
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
		if !g.host.Yield(g.yieldFrame(u)) {
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
	body, err := speakable.MarshalJSON()
	if err != nil {
		panic("koine/wire: an utterance could not render itself: " + err.Error())
	}
	frame, err := YieldFrame{Wire: Version, Type: speakable.EventType(), Body: body}.MarshalJSON()
	if err != nil {
		panic("koine/wire: a yield frame could not render itself: " + err.Error())
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
type Variant struct{ Name string }

func (v *Variant) Error() string { return v.Name }

// broker turns koine.Broker's three beats into the three host calls. It is
// the only implementation of Broker the SDK ships for a live host, and a
// station body can no more reach it than it can reach the host.
type broker struct{ host Host }

// pollLimit is a guard against a host that answers "pending" forever, not a
// budget. Budgets are the engine's, minted from the manifest's topology; a
// guest that spins a million times is not over budget, it is talking to
// something broken, and it says so instead of hanging the sandbox. A host
// that suspends the guest across the boundary never reaches one iteration.
// It is a var and not a const for exactly one reason: this package's own
// test lowers it to prove the guard fires. Nothing exported reaches it, and
// a guest cannot change it.
var pollLimit = 1 << 20

// Speak utters the intent and waits on nothing.
func (b broker) Speak(ex koine.Exchange) koine.Token {
	frame, err := ExchangeFrame{Wire: Version, Seat: ex.Seat, Name: ex.Name, Args: ex.Args}.MarshalJSON()
	if err != nil {
		panic("koine/wire: an exchange frame could not render itself: " + err.Error())
	}
	opened, err := DecodeOpened(b.host.Exchange(frame))
	if err != nil {
		panic("koine/wire: the host answered " + strconv.Quote(ex.Name) + " with a frame this build cannot read: " + err.Error())
	}
	if opened.Err != "" {
		// A seat that cannot open is not a domain outcome; it is a
		// deployment that registration should already have refused by
		// name. Trapping is the honest end — the host attributes it.
		panic("koine/wire: the host refused to open " + strconv.Quote(ex.Name) + " at seat " + strconv.Quote(ex.Seat) + ": " + opened.Err)
	}
	if opened.Token == 0 {
		panic("koine/wire: the host opened " + strconv.Quote(ex.Name) + " with no token — there would be nothing to gate on")
	}
	return koine.Token(opened.Token)
}

// Received is the fast beat. Pending answers a zero Ack: an honest not-yet.
func (b broker) Received(t koine.Token) koine.Ack {
	ack, err := DecodeAck(b.host.AckPoll(uint64(t)))
	if err != nil {
		panic("koine/wire: the host answered an ack poll with a frame this build cannot read: " + err.Error())
	}
	if ack.State != StateReceived {
		return koine.Ack{}
	}
	return koine.Ack{By: koine.ActorRef(ack.By)}
}

// Await waits until the exchange is filled or breached (E-C, amended
// 2026-08-27). The loop is deliberately the dumbest possible guest: it lets
// the HOST choose the mechanism. A host that suspends and resumes the guest
// across the boundary and a host that simply does not return until there is
// news are both conforming, and neither is a fact this package pins.
func (b broker) Await(t koine.Token) koine.Answer {
	for polls := 0; polls < pollLimit; polls++ {
		v, err := DecodeValue(b.host.ValuePoll(uint64(t)))
		if err != nil {
			panic("koine/wire: the host answered a value poll with a frame this build cannot read: " + err.Error())
		}
		switch v.State {
		case StateFilled:
			return koine.Answer{By: koine.ActorRef(v.By), JSON: v.Body}
		case StateBreached:
			return koine.Answer{By: koine.ActorRef(v.By), Err: &Variant{Name: v.Err}}
		case StatePending:
			continue
		default:
			panic("koine/wire: the host answered a value poll with the state " + strconv.Quote(v.State) + ", which is not one this contract admits")
		}
	}
	panic("koine/wire: the host answered pending " + strconv.Itoa(pollLimit) + " times — a value that never arrives and never breaches is a host that is not answering")
}

// ErrNoHost is what an off-target build of the host bindings answers with.
// It exists so that a native build of a guest fails loudly at the boundary
// rather than pretending to be in a sandbox.
var ErrNoHost = errors.New("koine/wire: there is no host below this build — a guest runs in the engine's sandbox, and nothing else is offered")
