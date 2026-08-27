package station

import (
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
)

// DeploymentAuditor stands at the executing stratum and exists to pin the
// concurrent pattern the steward does not use, written the way an author
// writes it — by silence, unbound and walked away from — so the extractor
// is read against real usage rather than a fixture posed for it.
type DeploymentAuditor struct{ koine.ExecutionBase }

// Identity is the claim this station carries.
func (DeploymentAuditor) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "deployment-auditor"}
}

// Awaits is the same completed thought the steward awaits — one shape, two
// comprehenders, no addressees.
func (DeploymentAuditor) Awaits() []selector.Selector {
	return selector.List(deployment.Resolved())
}

// Complete is the default shape of complete.
func (DeploymentAuditor) Complete() koine.Contract { return koine.DefaultAllAwaited }

// Resolve speaks one exchange and consumes it not at all.
func (a DeploymentAuditor) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)

	// Concurrent, the default: spoken and never consumed, so this station's
	// completion gates on the exchange resolving. Nothing is silently
	// ungated — the silence IS the gate.
	dep.History().Last(koine.Failure)

	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}
