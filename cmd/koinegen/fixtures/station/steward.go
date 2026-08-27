package station

import (
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
)

// DeploymentSteward is the design document's §10 worked example. It observes
// the completed deployment thought — either outcome, because the resolved
// idiom awaits the completed thought and the body branches — and speaks what
// comes next.
type DeploymentSteward struct{ koine.ObserverBase }

// Identity is the claim this station carries. The SDK carries it and never
// verifies it; the registration door checks it against the directory login
// already established.
func (DeploymentSteward) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "deployment-steward"}
}

// Awaits is the wiring: the expected graph IS the routing table, written
// before runtime.
func (DeploymentSteward) Awaits() []selector.Selector {
	return selector.List(deployment.Resolved())
}

// Complete is the default shape of complete: every awaited shape present.
func (DeploymentSteward) Complete() koine.Contract { return koine.DefaultAllAwaited }

// Resolve calculates or emits — the data is already there.
func (s DeploymentSteward) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	if dep.Outcome == koine.Success {
		yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
		return
	}
	// Inline: the value is consumed, so the exchange runs in this station's
	// own sequence — the same chain.
	lastGood, err := dep.History().Last(koine.Success).Value()
	if err != nil {
		// The typed outcome variant of the expected response. Branch on it;
		// there is nothing here to defend against.
		yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
		return
	}
	yield(deployment.Deploy{Artifact: lastGood.ArtifactID, Target: dep.Environment})
}
