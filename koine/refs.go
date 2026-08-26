package koine

// ChainRef is where a station stands in the flow — read-only from every
// stratum. Minted and continued by the host; a station observes its chain,
// it never invents one.
type ChainRef string

// ActorRef is whose authority a station carries (subject and, when acting on
// behalf, the acting party), minted by the host at construction. The station
// reads it; it cannot forge it.
type ActorRef string

// Outcome is a terminal's disposition. The resolved idiom delivers the
// COMPLETED thought regardless of outcome — the body branches on it, the
// declaration never filters by it.
type Outcome string

const (
	Success Outcome = "success"
	Failure Outcome = "failure"
)

// Evidence is what an adjudication stands on. Ref is a universal address
// into the record — one address scheme names everything, so evidence points
// at where true data already exists or there would be nothing to carry.
// Note is a sentence for a person.
type Evidence struct {
	Ref  string
	Note string
}

// Ack is the fast beat: someone who declared comprehension has received.
// By names the comprehender — the honest content of an acknowledgment,
// because only the party who declared comprehension receives.
type Ack struct {
	By ActorRef
}

// ProjectContext is the pinned interface that keeps a station body
// tool-free: the station speaks intents at it and the deployment answers.
// Its content is deliberately UNRULED here — the shape is reserved, and the
// ruling that fills it is surfaced, not invented (see the design document's
// escalation discipline). K0 pins only that the seat exists.
type ProjectContext interface{ projectContext() }
