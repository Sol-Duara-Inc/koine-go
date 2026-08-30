// Package koinetest is the author's only test surface, and it is complete.
//
// Resolve is a pure function from delivery to utterances, so a station is
// testable by calling it and collecting the yields — no engine, no sandbox,
// no network, no daemon, and none of those are offered here. An author who
// wants to test against a live engine is on the deployment's side of the
// line, with the deployment's own tools; this SDK does not carry that door,
// and it never will: shipping a test affordance against a live surface keeps
// everyone wanting to use it.
//
// Because a history fulfiller answers from the record and the SDK never
// caches, a station is fully deterministic given its delivery plus its
// scripted exchanges. This harness is therefore not a mock of the semantics.
// It IS the semantics minus the transport.
//
//	out := koinetest.Run(&DeploymentSteward{},
//	        koinetest.Deliver(deployment.ResolvedDelivery{Outcome: koine.Failure}),
//	        koinetest.Exchange("history.last", lastGood))
//	// assert on out.Utterances, out.Exchanges, out.Consumption
package koinetest

import (
	"fmt"
	"sort"

	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/manifest"
	"github.com/sol-duara-inc/koine-go/koine/selector"
)

// Marshaler is what a scripted answer must be: a generated domain type,
// which knows how to render itself without reflection. Anything else is not
// a thing the record could have answered with.
type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

// Out is everything one run said about itself.
type Out struct {
	// Identity, Awaits and Complete are the station's declaration, read
	// back so a test can assert on the wiring as well as the speech.
	Identity koine.Identity
	Awaits   []selector.Selector
	Complete koine.Contract

	// Utterances are the yields, in the order they were spoken. Emitting
	// is the storage action; this slice is what the record would hold.
	Utterances []koine.Utterance

	// Exchanges are the intents spoken, in the order they were uttered.
	Exchanges []Spoken

	// Passages are the hand-offs to the parent, in order. A station that
	// wrote no verbs and declared no hooks makes none of its own: the
	// step-end default pass is the host duty, and this slice being empty
	// is how a run says so.
	Passages []Passage

	// Consumption is how each exchange was treated, by exchange name.
	// Both patterns are witnessed, not inferred: Value() reports itself
	// through the broker. A run and the manifest koinegen derived can
	// therefore be compared word for word, which is what makes A3 a gate
	// rather than a gesture.
	Consumption map[string]manifest.Consumption
}

// Spoken is one exchange as the run saw it.
type Spoken struct {
	Seat string
	Name string
	Args []koine.Arg
	// Consumption is this exchange's pattern, in the same vocabulary the
	// manifest uses — one word for one fact, at both altitudes.
	Consumption manifest.Consumption
	// Err is the typed outcome variant the script answered with.
	Err error
}

// Option scripts one part of a run.
type Option func(*run)

// Deliver is the projected facts the station is constructed with. Exactly
// one delivery is required: a station resolves from a delivery, and a run
// without one has nothing to be a function of.
func Deliver(d koine.Delivery) Option {
	return func(r *run) { r.delivery = d }
}

// Exchange scripts the answer to one spoken exchange, named the way the
// generated verb names it — "history.last".
func Exchange(name string, answer Marshaler) Option {
	return func(r *run) { r.script[name] = scripted{answer: answer} }
}

// ExchangeFails scripts the typed outcome variant of an expected response:
// the future went the other way. It is never transport — there is no
// transport here — so a station that branches on it is branching on domain
// truth, which is the only thing it could ever branch on in production.
func ExchangeFails(name string, err error) Option {
	return func(r *run) { r.script[name] = scripted{err: err} }
}

// StopAfter makes the yield refuse after n utterances, the way a host that
// cancelled would. Nothing after the refusal is spoken.
func StopAfter(n int) Option {
	return func(r *run) { r.stopAfter = n }
}

// Standing is where the host says this station stands: the chain it is in and
// the actor whose authority it carries. Construction is delivery, and a run
// that skipped it would hand the body an empty Base and call that a test.
func Standing(chain koine.ChainRef, actor koine.ActorRef) Option {
	return func(r *run) { r.chain, r.actor = chain, actor }
}

// Parent scripts what this station's parent concludes when its step ends. A
// run that never sets one answers a plain success, which is what a parent
// that enriched and stored concludes.
func Parent(c koine.Conclusion) Option {
	return func(r *run) { r.parent = c }
}

type scripted struct {
	answer Marshaler
	err    error
}

// run is the harness's own broker and collector. It is the only Broker in
// this repository: the SDK ships no path to a running engine.
type run struct {
	delivery   koine.Delivery
	script     map[string]scripted
	stopAfter  int
	chain      koine.ChainRef
	actor      koine.ActorRef
	parent     koine.Conclusion
	utterances []koine.Utterance
	spoken     []*Spoken
	passages   []*Passage
	// passing is koine's one-passage gate, the same one the sandbox
	// enforces. The bench holds stations to the rule the engine holds
	// them to, or it is not the semantics minus the transport.
	passing koine.Passing
}

// Passage is one hand-off to the parent as the run saw it: what went up, or
// what was withheld, and whether the body waited for the answer.
type Passage struct {
	// Offered is the object handed to the parent, or withheld from it.
	Offered koine.Utterance
	// Withheld is the only suppression. It is recorded because
	// suppressing a feature the fleet agreed on is a fact a test should
	// be able to see.
	Withheld bool
	// Awaited says the body asked for the parent conclusion — which is
	// how a station declares it will handle what comes back.
	Awaited bool
}

// Run drives the station and returns everything it said.
func Run(k koine.Koine, opts ...Option) Out {
	r := &run{
		script:    map[string]scripted{},
		stopAfter: -1,
		parent:    koine.Conclusion{Outcome: koine.Success},
	}
	for _, opt := range opts {
		opt(r)
	}
	if r.delivery == nil {
		panic("koinetest: Run needs koinetest.Deliver(...) — a station resolves from a delivery, and there is nothing here to be a function of")
	}
	// Construction is delivery (§4). The harness IS the semantics minus
	// the transport, so it builds the station the way a host does —
	// otherwise a body that reads its chain, or speaks to its parent,
	// would find an empty Base and the test would be testing a station
	// nobody constructed.
	if !koine.Construct(k, koine.Standing{Chain: r.chain, Actor: r.actor, Lineage: r}) {
		panic("koinetest: Run could not construct this station. Take its address — koinetest.Run(&Steward{}, ...) — because standard parts are state and state written into a copy is state nobody can read. A station that is not addressable is one the host could not construct either, and a harness that let you test it would not be the semantics minus the transport.")
	}

	d := r.delivery
	if b, ok := d.(koine.Bindable); ok {
		d = b.Bind(r)
	}

	k.Resolve(d, func(u koine.Utterance) bool {
		if r.stopAfter >= 0 && len(r.utterances) >= r.stopAfter {
			return false
		}
		r.utterances = append(r.utterances, u)
		return true
	})

	// The named hooks, run exactly as the sandbox runs them — same
	// function, same order. A harness that ran its own version of this
	// would be a harness that could disagree with the engine about what a
	// hook does, which is the one thing it must never do.
	if err := koine.RunHooks(k, d, r); err != nil {
		// The author's own shape mistake, said the way the sandbox says
		// it: a station that declared Post without Pre cannot run.
		panic("koinetest: " + err.Error())
	}

	out := Out{
		Identity:    k.Identity(),
		Awaits:      k.Awaits(),
		Complete:    k.Complete(),
		Utterances:  r.utterances,
		Consumption: map[string]manifest.Consumption{},
	}
	for _, s := range r.spoken {
		out.Exchanges = append(out.Exchanges, *s)
		if out.Consumption[s.Name] != manifest.Inline {
			out.Consumption[s.Name] = s.Consumption
		}
	}
	for _, p := range r.passages {
		out.Passages = append(out.Passages, *p)
	}
	return out
}

// Speak opens a spoken exchange from the script and waits on nothing — the
// utterance leaves, and the token names it. An exchange nobody scripted is a
// loud failure, never an invented answer: the run would otherwise be
// asserting on a fact the author never stated, and a seat nobody filled is
// exactly what registration exists to refuse by name.
func (r *run) Speak(ex koine.Exchange) koine.Token {
	s, ok := r.script[ex.Name]
	if !ok {
		panic(fmt.Sprintf("koinetest: nothing scripted for exchange %q (seat %q) — script it with koinetest.Exchange or koinetest.ExchangeFails; a harness that invented the answer would be asserting on a fact you never stated. Scripted: %v", ex.Name, ex.Seat, r.scriptedNames()))
	}
	spoken := &Spoken{Seat: ex.Seat, Name: ex.Name, Args: ex.Args, Consumption: manifest.Concurrent, Err: s.err}
	r.spoken = append(r.spoken, spoken)
	return koine.Token(len(r.spoken)) // one-based; zero is no exchange at all
}

// Received answers the fast beat. A scripted exchange is comprehended the
// moment it is spoken, because the harness IS the fulfiller: there is no
// transport here to be slow.
func (r *run) Received(t koine.Token) koine.Ack {
	s := r.at(t)
	if s == nil {
		return koine.Ack{}
	}
	return koine.Ack{By: koine.ActorRef("koinetest:" + s.Seat)}
}

// Await waits until the exchange is filled or breached — which, against a
// script, is immediately. Being asked to await IS the consumption beat: this
// exchange ran in the station's own sequence, inline.
func (r *run) Await(t koine.Token) koine.Answer {
	s := r.at(t)
	if s == nil {
		panic(fmt.Sprintf("koinetest: awaited token %d, which names no exchange this run opened", t))
	}
	s.Consumption = manifest.Inline
	by := koine.ActorRef("koinetest:" + s.Seat)
	scripted := r.script[s.Name]
	if scripted.err != nil {
		return koine.Answer{By: by, Err: scripted.err}
	}
	data, err := scripted.answer.MarshalJSON()
	if err != nil {
		panic(fmt.Sprintf("koinetest: the answer scripted for %q could not render itself: %v", s.Name, err))
	}
	return koine.Answer{By: by, JSON: data}
}

func (r *run) at(t koine.Token) *Spoken {
	if t == 0 || int(t) > len(r.spoken) {
		return nil
	}
	return r.spoken[t-1]
}

func (r *run) scriptedNames() []string {
	names := make([]string, 0, len(r.script))
	for name := range r.script {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// PassUp records the hand-off and returns a passage. koinetest is the only
// Lineage this SDK ships beside the wire's, and like the wire's it invents no
// answer: what the parent concludes is what the test said it concludes.
func (r *run) PassUp(u koine.Utterance) koine.Passage {
	if !r.passing.Offer() {
		return 0 // withheld; the pass is what suppression suppresses
	}
	r.passages = append(r.passages, &Passage{Offered: u})
	return koine.Passage(len(r.passages)) // one-based; zero is no passage at all
}

// Withhold records the suppression. Nothing goes up.
func (r *run) Withhold(u koine.Utterance) {
	if !r.passing.Suppress() {
		return
	}
	r.passages = append(r.passages, &Passage{Offered: u, Withheld: true})
}

// AwaitPass answers what the test said the parent concludes — except for a
// suppressed passage, where the answer is single-sourced beside the gate
// (koine.Passing.AwaitedConclusion) so the bench and the sandbox cannot
// drift about what awaiting a withheld pass means.
func (r *run) AwaitPass(p koine.Passage) koine.Conclusion {
	if p == 0 {
		if c, ok := r.passing.AwaitedConclusion(); ok {
			return c
		}
	}
	if p >= 1 && int(p) <= len(r.passages) {
		r.passages[p-1].Awaited = true
	}
	return r.parent
}
