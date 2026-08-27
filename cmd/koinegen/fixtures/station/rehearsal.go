package station

import (
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
)

// DeploymentRehearsal writes the same intents as the steward in the other
// legal spelling: it BINDS its handle and its seat before speaking through
// them. Binding is not a style — it is the only way to reach Received(), the
// other half of the ratified Handle contract — so an analyzer that read the
// chained spelling and nothing else would call this station's inline
// exchange a blocking one and hand the engine a sibling chain to budget for
// work that runs serially. The fixture exists so that reading is pinned.
type DeploymentRehearsal struct{ koine.ObserverBase }

// Identity is the claim this station carries.
func (DeploymentRehearsal) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "deployment-rehearsal"}
}

// Awaits is the same completed thought its siblings await.
func (DeploymentRehearsal) Awaits() []selector.Selector {
	return selector.List(deployment.Resolved())
}

// Complete is the default shape of complete.
func (DeploymentRehearsal) Complete() koine.Contract { return koine.DefaultAllAwaited }

// Resolve speaks through names rather than through chains.
func (r DeploymentRehearsal) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)

	// Bound, gated on the fast beat, then consumed. Consuming the value is
	// what makes it inline, wherever the handle was named.
	lastGood := dep.History().Last(koine.Success)
	if lastGood.Received().By == "" {
		yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
		return
	}
	good, err := lastGood.Value()
	if err != nil {
		yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
		return
	}

	// A seat bound to a name, spoken through the name — and never consumed,
	// so this station's completion gates on it.
	ledger := dep.Ledger()
	ledger.Note("rehearsed " + string(good.ArtifactID))

	yield(deployment.Deploy{Artifact: good.ArtifactID, Target: dep.Environment})
}
