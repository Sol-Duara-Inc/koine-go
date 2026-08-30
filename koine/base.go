package koine

import "time"

// hostContext is the host-mediated construction context. The host builds the
// station; authors never construct one of these, and there is nothing here
// for them to reach around.
type hostContext struct {
	chain   ChainRef
	actor   ActorRef
	project ProjectContext
	lineage Lineage
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
// an opinion, and the record carries no opinions.
//
// It panics, and as of wire v1 it panics everywhere, because there is
// NOTHING BELOW IT TO CALL: the host's guest-visible surface is six
// functions (yield, exchange, ack_poll, value_poll, deliver, host_log) and
// none of them adjudicates a chain. Chain verbs are K3's — "exchanges and
// branching" — and until that lands, a loud panic is the only honest
// answer. The alternative, doing nothing quietly, would be a station
// believing it had disposed of a chain that is still running.
func (e *ExecutionBase) ResolveChain(outcome Outcome, evidence ...Evidence) {
	panic("koine: ResolveChain has no host function to reach — chain verbs land in K3, and a chain verb never fails silently")
}

// ExtendChain re-arms the chain's clock within engine policy. The host caps
// the extension; the station asks, it does not decide. Like ResolveChain it
// has nothing below it to call in wire v1, and says so loudly rather than
// letting a station believe its clock was re-armed.
func (e *ExecutionBase) ExtendChain(within time.Duration) {
	panic("koine: ExtendChain has no host function to reach — chain verbs land in K3, and a chain verb never fails silently")
}
