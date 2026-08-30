package station

import (
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
)

// The pass-up fixtures. ChainVerbs and ChainHooks are TWINS: they hand their
// parent the same object and ask for the same answer, one by writing the
// verbs and one by declaring the named hooks. The hooks are sugar over the
// verbs and nothing else, so the traffic the two produce must be identical —
// which is what a test can hold them to.
//
// What they do NOT share is position, and that is the substrate rather than a
// discrepancy: the verb form passes up where the author put the line, and the
// hook form passes up at step end, which is the default position. An author
// who needs the parent to have the object earlier writes the line.

// ChainVerbs writes the verbs. The parent gets the object, the child keeps
// working, and the child asks for the answer when it wants it.
type ChainVerbs struct{ koine.ObserverBase }

// Identity is the claim this station carries.
func (ChainVerbs) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "chain-verbs"}
}

// Awaits is the completed deployment thought.
func (ChainVerbs) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }

// Complete is the default shape of complete.
func (ChainVerbs) Complete() koine.Contract { return koine.DefaultAllAwaited }

// Resolve passes up, keeps working, then asks.
func (s *ChainVerbs) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)

	// Before the parent: the child's own knowledge, added here. It is
	// built inline rather than in a helper because the manifest analyzer
	// reads THIS body and no other — a delivery handed to a helper is
	// speech that never reaches the manifest, and it refuses that by name.
	offered := deployment.DeploymentRecorded{Artifact: dep.ArtifactID}

	// The parent has it from this line onward.
	passage := s.PassUp(offered)

	// After the parent: whatever the child concluded on its own.
	if got := s.Await(passage); got.Err != nil {
		// A parent finding is a VALUE. Branch on it.
		yield(deployment.Deploy{Artifact: dep.ArtifactID, Target: dep.Environment})
		return
	}
	yield(offered)
}

// ChainHooks declares the named hooks and writes no verb at all. Pre mints
// what goes up; Post is told what came back.
type ChainHooks struct {
	koine.ObserverBase
	concluded koine.Conclusion
}

// Identity is the claim this station carries.
func (ChainHooks) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "chain-hooks"}
}

// Awaits is the completed deployment thought.
func (ChainHooks) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }

// Complete is the default shape of complete.
func (ChainHooks) Complete() koine.Contract { return koine.DefaultAllAwaited }

// Resolve says nothing about the parent. The hooks do that.
func (s *ChainHooks) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}

// Pre mints what goes up — the same object ChainVerbs builds by hand.
func (ChainHooks) Pre(d koine.Delivery) koine.Utterance {
	dep := d.(deployment.ResolvedDelivery)
	return deployment.DeploymentRecorded{Artifact: dep.ArtifactID}
}

// Post is told what the parent concluded. Declaring it is declaring the
// await, which is why the twin's Await has no counterpart written here.
func (s *ChainHooks) Post(u koine.Utterance, c koine.Conclusion) { s.concluded = c }

// Concluded is what Post was told, for a test to read.
func (s *ChainHooks) Concluded() koine.Conclusion { return s.concluded }

// ChainWithholds suppresses the pass the fleet would otherwise make. It is
// the only opt-out there is, and it is written down, in this file, under this
// author's name.
type ChainWithholds struct{ koine.ObserverBase }

// Identity is the claim this station carries.
func (ChainWithholds) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "chain-withholds"}
}

// Awaits is the completed deployment thought.
func (ChainWithholds) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }

// Complete is the default shape of complete.
func (ChainWithholds) Complete() koine.Contract { return koine.DefaultAllAwaited }

// Resolve withholds, and says why in the only place that matters — the code.
func (s *ChainWithholds) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	s.Withhold(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}

// ChainWalker speaks all three verbs across its two branches, so one compiled
// guest can carry the whole surface into the engine's sandbox. The branches
// are not decoration: a station that withholds on one path and passes up on
// another is the ordinary shape of "features are opt-out, and opting out is
// a decision made about a particular event".
type ChainWalker struct{ koine.ExecutionBase }

// Identity is the claim this station carries.
func (ChainWalker) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "chain-walker"}
}

// Awaits is the completed deployment thought.
func (ChainWalker) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }

// Complete is the default shape of complete.
func (ChainWalker) Complete() koine.Contract { return koine.DefaultAllAwaited }

// Resolve passes up and waits when the deployment failed, and withholds when
// it succeeded — there is nothing for the parent to add to a success this
// station already recorded.
func (s *ChainWalker) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	offered := deployment.DeploymentRecorded{Artifact: dep.ArtifactID}

	if dep.Outcome == koine.Success {
		s.Withhold(offered)
		yield(offered)
		return
	}

	passage := s.PassUp(offered)
	yield(offered) // the child keeps working while the parent has it

	if got := s.Await(passage); got.Err != nil {
		yield(deployment.Deploy{Artifact: dep.ArtifactID, Target: dep.Environment})
	}
}
