package koine

import "errors"

// The pass-up: how a controller hands its parent the event object as the walk
// happens, with its own tribal knowledge running before, after, or around the
// parent's feature.
//
// Ratified 2026-08-30 (conduit-go docs/truth/frames/koine-station-model.md
// §THE PASS-UP MECHANISM), from the owner's requirement: "sometimes the
// tribal knowledge will need to be executed before the parent, sometimes
// after, and frequently a bit of both."
//
// THE SUBSTRATE IS CODE POSITION. "A bit of both" is literally lines above
// and below the PassUp line — so the mechanism is not a callback table, it is
// an awaitable hand-off you place where you want it:
//
//	func (s Child) Resolve(d koine.Delivery, yield koine.Yield) {
//	    dep := d.(deployment.ResolvedDelivery)
//	    enriched := s.enrich(dep)          // before the parent
//	    p := s.PassUp(enriched)            // the parent gets it NOW
//	    yield(...)                         // the child keeps working
//	    got := s.Await(p)                  // ...until it wants the outcome
//	    if got.Outcome == koine.Failure {  // after the parent
//	        yield(...)
//	    }
//	}
//
// Features are OPT-OUT: an author who writes none of the three verbs still
// passes up, because the host does it at step end. Withhold is the only
// suppression, and a child TRAP is not one — "a bug is not an opt-out",
// because the alternative makes every vendor feature fragile to every
// customer bug.
//
// # Awaiting IS declaring
//
// The law says the parent's outcome reaches the child as a VALUE "where
// declared" and unwinds where not. This SDK reads Await itself as the
// declaration, and invents no second surface to say the same thing twice.
// That is not a convenience: §6's branch control already COMPILES FROM
// USAGE — a value consumed is inline, a value never consumed is a gate — and
// a station that asked for its parent's outcome has said, in code, that it
// intends to handle it. A station that never awaits never receives one, so
// there is nothing in the body for a breach to arrive at, and the unwind is
// the walk's, host-side, where it always was.
//
// This reading is the SDK's, and it is marked as such: see koine/wire's
// PROPOSED SPELLINGS block for everything about the pass-up that the two
// halves of this contract have not yet agreed in writing.

// Passage is one pass-up the host opened: the child's claim on its parent's
// outcome. It is deliberately NOT Token — a passage and a spoken exchange are
// both host-minted handles, and a type that let one be awaited as the other
// would be a type that permits a category error.
type Passage uint64

// Conclusion is what the parent's step concluded, handed back as a value.
type Conclusion struct {
	// Outcome is the parent's terminal disposition. The resolved idiom
	// delivers the completed thought either way; a child branches on it.
	Outcome Outcome
	// Err is the parent's finding when there was one: a tool breach the
	// parent reported, or an unfilled parent — a namespace declared and
	// not loaded. It is a VALUE, not a panic. Branch on it.
	Err error
}

// OK reports the plain case: the parent's step concluded and found nothing to
// report.
func (c Conclusion) OK() bool { return c.Err == nil && c.Outcome != Failure }

// Lineage is the seam the pass-up verbs speak through: the parent chain,
// which the law calls "the first real broker". The engine implements it below
// the line over koine/wire; koine/testing implements it from a script.
//
// A station body never holds one. It holds a Base, and Base's three verbs are
// the only door — the same arrangement as every other host-mediated thing in
// this SDK.
//
// AwaitPass rather than Await, because koine.Broker already declares
// Await(Token) Answer and one interface cannot carry two methods of one name.
// The AUTHOR still writes Await: Base.Await is the verb, and this is the seam
// under it.
type Lineage interface {
	// PassUp hands the parent its object now and returns without waiting.
	PassUp(Utterance) Passage
	// AwaitPass suspends until the parent's step concludes. The host does
	// the waiting; a guest never spins.
	AwaitPass(Passage) Conclusion
	// Withhold suppresses the default pass. It is the only suppression.
	Withhold(Utterance)
}

// PassUp hands the parent its object NOW and returns a Passage without
// waiting. Everything the body does after this line runs while the parent
// has already been handed the object — which is what makes "a bit of both"
// a matter of where the line sits.
func (b *Base) PassUp(u Utterance) Passage {
	return b.lineage("PassUp").PassUp(u)
}

// Await suspends until the parent's step concludes and hands back its
// outcome as a VALUE — including a breach the parent reported. Asking is
// declaring: a station that awaits has said in code that it intends to
// handle whatever comes back.
//
// The waiting is the HOST's. There is no spin here and no timeout invented
// here, for the same reason koine/wire's exchange Await has neither: a budget
// is the engine's, and a guest that made up its own would be a guest arguing
// with the calculus.
func (b *Base) Await(p Passage) Conclusion {
	return b.lineage("Await").AwaitPass(p)
}

// Withhold suppresses the pass this station would otherwise make. It is the
// ONLY suppression: a trap does not suppress, an early return does not
// suppress, and an outcome of failure does not suppress. Features are
// opt-out, and opting out is written down.
func (b *Base) Withhold(u Utterance) {
	b.lineage("Withhold").Withhold(u)
}

// lineage answers the seam, or refuses loudly. A nil seam means the station
// was never constructed — a wiring fault in whoever built it, not a runtime
// condition a body could branch on — so this is the same posture the
// generated verbs take for an unbound delivery: say so, once, by name.
func (b *Base) lineage(verb string) Lineage {
	if b.ctx.lineage == nil {
		panic("koine: " + verb + " spoke with no parent chain below this station — a station reaches its lineage through construction, and this one was never constructed")
	}
	return b.ctx.lineage
}

// PreHook is a station that runs tribal knowledge BEFORE the parent consumes.
// It MINTS what goes up, from what the station was given — which is what "may
// transform" means when the thing being transformed is an object the child
// has not built yet. Declaring the method is what makes a station one:
//
//	func (s Child) Pre(d koine.Delivery) koine.Utterance { ... }
//
// It is an interface rather than a method on Base, and that is forced rather
// than chosen. A Pre on Base is promoted to EVERY station, so every station
// would satisfy it and the guest could no longer tell a station that wrote a
// hook from one that wrote nothing — which is exactly the distinction the
// zero-code default rests on. A sentinel flag set by Base's default does not
// rescue it either: this SDK's own inheritance law says the parent runs by
// default and an author's shadowing Pre may call the embedded Base's Pre, so
// the flag would be set by the very idiom the design encourages. Named hooks
// are the ruling; interfaces are the only spelling of them that keeps the
// ruling's other half true.
type PreHook interface {
	Pre(Delivery) Utterance
}

// PostHook is a station that runs after the parent's step concludes.
// Declaring the method is what makes it one:
//
//	func (s Child) Post(u koine.Utterance, c koine.Conclusion) { ... }
//
// Declaring Post is declaring an await: the guest awaits the passage so it
// has a Conclusion to hand you.
//
// Post requires Pre. A station that declared only Post has asked to be told
// what its parent concluded about an object it never minted, and the guest
// has nothing to pass up on its behalf; koinegen refuses that station by
// name rather than letting it discover the gap at run time.
type PostHook interface {
	Post(Utterance, Conclusion)
}

// ErrPostWithoutPre is a station that declared Post and not Pre: it asked
// what its parent concluded about an object it never minted.
var ErrPostWithoutPre = errors.New("koine: this station declares Post without Pre — it asked what its parent concluded about an object it never minted, and nothing can mint one for it")

// RunHooks runs the named hooks as SUGAR over the three verbs, and is the
// only implementation of that sugar in this repository.
//
// It lives here, beside Construct, for the same reason Construct does: the
// host calls it and an author never does. It is exported because there are
// TWO hosts — koine/wire in the sandbox and koine/testing on the author's
// bench — and a second copy of this sequence is a second thing to drift. The
// harness is the semantics minus the transport; that is only true if the
// semantics are literally the same code.
//
// Everything below is a call to the same three verbs an author could have
// written by hand, in the same order, with the same arguments. That is what
// makes hook-form and verb-form indistinguishable from the parent's side.
//
// It runs at STEP END, because that is the default pass position — "the
// child's tribal knowledge precedes the vendor's generic feature; enrichment
// naturally comes first". An author who needs the parent to have the object
// earlier writes PassUp where they want it, which is the substrate the hooks
// are sugar over.
func RunHooks(k Koine, d Delivery, l Lineage) error {
	pre, hasPre := k.(PreHook)
	post, hasPost := k.(PostHook)
	if !hasPre && !hasPost {
		return nil // zero code, zero traffic
	}
	if !hasPre {
		return ErrPostWithoutPre
	}
	offered := pre.Pre(d)
	if offered == nil {
		// Pre answering nothing is a station declining to hand anything
		// over, said in the hook form. It is NOT a Withhold: withholding
		// is written down, and this is simply nothing to pass.
		return nil
	}
	passage := l.PassUp(offered)
	if hasPost {
		post.Post(offered, l.AwaitPass(passage))
	}
	return nil
}
