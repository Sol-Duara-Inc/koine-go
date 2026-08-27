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
//	out := koinetest.Run(DeploymentSteward{},
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
	utterances []koine.Utterance
	spoken     []*Spoken
	byName     map[string]*Spoken
}

// Run drives the station and returns everything it said.
func Run(k koine.Koine, opts ...Option) Out {
	r := &run{script: map[string]scripted{}, stopAfter: -1, byName: map[string]*Spoken{}}
	for _, opt := range opts {
		opt(r)
	}
	if r.delivery == nil {
		panic("koinetest: Run needs koinetest.Deliver(...) — a station resolves from a delivery, and there is nothing here to be a function of")
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
	return out
}

// Speak answers a spoken exchange from the script. An exchange nobody
// scripted is a loud failure, never an invented answer: the run would
// otherwise be asserting on a fact the author never stated.
func (r *run) Speak(ex koine.Exchange) koine.Answer {
	s, ok := r.script[ex.Name]
	if !ok {
		panic(fmt.Sprintf("koinetest: nothing scripted for exchange %q (seat %q) — script it with koinetest.Exchange or koinetest.ExchangeFails; a harness that invented the answer would be asserting on a fact you never stated. Scripted: %v", ex.Name, ex.Seat, r.scriptedNames()))
	}
	spoken := &Spoken{Seat: ex.Seat, Name: ex.Name, Args: ex.Args, Consumption: manifest.Concurrent, Err: s.err}
	r.spoken = append(r.spoken, spoken)
	r.byName[ex.Name] = spoken
	if s.err != nil {
		return koine.Answer{By: koine.ActorRef("koinetest:" + ex.Seat), Err: s.err}
	}
	data, err := s.answer.MarshalJSON()
	if err != nil {
		panic(fmt.Sprintf("koinetest: the answer scripted for %q could not render itself: %v", ex.Name, err))
	}
	return koine.Answer{By: koine.ActorRef("koinetest:" + ex.Seat), JSON: data}
}

// Consumed records that the body materialized this exchange's value: the
// exchange ran in the caller's own sequence, inline.
func (r *run) Consumed(ex koine.Exchange) {
	if s := r.byName[ex.Name]; s != nil {
		s.Consumption = manifest.Inline
	}
}

func (r *run) scriptedNames() []string {
	names := make([]string, 0, len(r.script))
	for name := range r.script {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
