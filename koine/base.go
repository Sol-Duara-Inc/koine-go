package koine

import "time"

// hostContext is the host-mediated construction context. The host builds the
// station; authors never construct one of these, and there is nothing here
// for them to reach around.
type hostContext struct {
	chain   ChainRef
	actor   ActorRef
	project ProjectContext
}

// Base carries the standard parts every station stands on. Embed it through
// one of the stratum bases below — never directly. Construction is delivery:
// the host builds the station; authors never call a constructor.
type Base struct{ ctx hostContext }

// Chain is where I stand in the flow. Read-only, every stratum.
func (b *Base) Chain() ChainRef { return b.ctx.chain }

// Actor is whose authority I carry, minted by the host.
func (b *Base) Actor() ActorRef { return b.ctx.actor }

// Project is the pinned ProjectContext, projected for this station.
func (b *Base) Project() ProjectContext { return b.ctx.project }

// ObserverBase is the observing stratum. It hears like everyone and holds no
// chain verbs — not gated, not permission-checked: the verbs do not exist
// from this position, and a station that tries to speak one does not
// compile. Where a path must not exist, the affordance is removed, never
// guarded.
type ObserverBase struct{ Base }

// ExecutionBase is the executing stratum: plugin authors, workflow owners.
// It carries the chain verbs.
type ExecutionBase struct{ Base }

// ResolveChain adjudicates the chain. Evidence is required — the host
// refuses an empty adjudication — because a disposition without evidence is
// an opinion, and the record carries no opinions. With no host below this
// station the call panics by design: a chain verb never fails silently.
func (e *ExecutionBase) ResolveChain(outcome Outcome, evidence ...Evidence) {
	panic("koine: ResolveChain spoke with no host below this station — a chain verb never fails silently")
}

// ExtendChain re-arms the chain's clock within engine policy. The host caps
// the extension; the station asks, it does not decide. With no host below,
// it panics — loud, like every verb here.
func (e *ExecutionBase) ExtendChain(within time.Duration) {
	panic("koine: ExtendChain spoke with no host below this station — a chain verb never fails silently")
}
