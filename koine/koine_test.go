// These tests live in koine_test — a FOREIGN package on purpose. Generated
// strata live in their own packages, so everything a stranger package must
// be able to do (speak, be delivered, implement the station contract) is
// witnessed from outside the wall.
package koine_test

import (
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
)

// A domain vocabulary, authored outside the koine package — the way every
// generated stratum will be.
type deployRecorded struct {
	koine.IsUtterance
	Artifact string
}

type deploy struct {
	koine.IsUtterance
	Artifact, Target string
}

type deploymentResolved struct {
	koine.IsDelivery
	Outcome     koine.Outcome
	ArtifactID  string
	Environment string
}

func TestMarkers_CrossPackageStrata(t *testing.T) {
	var u koine.Utterance = deployRecorded{Artifact: "payments-api"}
	var d koine.Delivery = deploymentResolved{Outcome: koine.Success}
	if u == nil || d == nil {
		t.Fatal("markers did not carry the contract across the package wall")
	}
}

// The steward from the design document's worked example, reduced to K0's
// vocabulary: an observer station, written here as any author would write
// one.
type steward struct{ koine.ObserverBase }

func (steward) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "deployment-steward"}
}
func (steward) Awaits() []selector.Selector {
	return selector.List(selector.Resolved("dev.cdevents.deployment").At("deploy"))
}
func (steward) Complete() koine.Contract { return koine.DefaultAllAwaited }
func (steward) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deploymentResolved)
	if dep.Outcome == koine.Success {
		yield(deployRecorded{Artifact: dep.ArtifactID})
		return
	}
	yield(deploy{Artifact: dep.ArtifactID, Target: dep.Environment})
}

var _ koine.Koine = steward{} // one uniform interface, satisfied from a foreign package

// The paradigm's testability promise, pinned before any harness exists:
// "a Koine is testable by calling it and collecting the yields, no platform
// required."
func TestStation_ResolveIsAFunctionFromDeliveryToUtterances(t *testing.T) {
	var spoke []koine.Utterance
	collect := func(u koine.Utterance) bool { spoke = append(spoke, u); return true }

	steward{}.Resolve(deploymentResolved{Outcome: koine.Success, ArtifactID: "payments-api"}, collect)
	if len(spoke) != 1 {
		t.Fatalf("success branch spoke %d utterances, want 1", len(spoke))
	}
	if rec, ok := spoke[0].(deployRecorded); !ok || rec.Artifact != "payments-api" {
		t.Fatalf("success branch spoke %#v", spoke[0])
	}

	spoke = nil
	steward{}.Resolve(deploymentResolved{Outcome: koine.Failure, ArtifactID: "payments-api", Environment: "prod"}, collect)
	if len(spoke) != 1 {
		t.Fatalf("failure branch spoke %d utterances, want 1", len(spoke))
	}
	if dep, ok := spoke[0].(deploy); !ok || dep.Target != "prod" {
		t.Fatalf("failure branch spoke %#v", spoke[0])
	}
}

func TestContract_DefaultAllAwaitedIsTheZeroContract(t *testing.T) {
	if koine.DefaultAllAwaited == nil {
		t.Fatal("DefaultAllAwaited is nil")
	}
	c := koine.DefaultAllAwaited
	if c != koine.DefaultAllAwaited {
		t.Fatal("DefaultAllAwaited is not a comparable sentinel")
	}
}

func TestExecution_ChainVerbsAreLoudWithoutHost(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("ResolveChain with no host must panic — a chain verb never fails silently")
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "never fails silently") {
			t.Fatalf("panic said %v", r)
		}
	}()
	(&koine.ExecutionBase{}).ResolveChain(koine.Failure, koine.Evidence{Ref: "run/x", Note: "test"})
}
