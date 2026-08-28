package koine

// Construction is delivery: the host builds the station (§4). This file is
// the seam that makes that sentence true, and it is the whole of it.
//
// K0 wrote the standard parts and their readers — Chain, Actor, Project —
// but shipped no way for a host to FILL them, because K0 had no host. K2
// does: koine/wire hands a station the chain it stands in and the actor
// whose authority it carries, both minted below the line, once, before
// Resolve runs. Without this seam the wire would carry those two facts
// across the boundary and then have nowhere to put them, which is a
// contract that lies about itself.
//
// The seam is closed by construction. standing is unexported and its only
// implementation is *Base, so no station outside this package can fill its
// own standard parts and no author can reach a constructor: the only way to
// stand in a chain is to be handed one.

type standing interface{ standing() *Base }

func (b *Base) standing() *Base { return b }

// Construct fills a station's standard parts and reports whether it could.
// It answers false for a station that does not embed a stratum base — which
// is not a station, whatever else it is.
//
// The host calls this once per delivery, before Resolve. A station author
// never calls it: reaching it requires a ChainRef and an ActorRef, and those
// are minted below the line where author code cannot go.
//
// Note that a station must be addressable to be constructed. A host holds
// *DeploymentSteward, not DeploymentSteward — the standard parts are state,
// and state written into a copy is state nobody can read.
func Construct(k Koine, chain ChainRef, actor ActorRef, project ProjectContext) bool {
	s, ok := k.(standing)
	if !ok {
		return false
	}
	b := s.standing()
	b.ctx = hostContext{chain: chain, actor: actor, project: project}
	return true
}
