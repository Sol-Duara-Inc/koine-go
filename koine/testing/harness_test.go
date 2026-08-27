// These tests drive real fixture stations through the harness — the same
// stations koinegen's extractor reads. Nothing here is a stub: the claim is
// that this harness IS the semantics minus the transport, and a stub would
// be a mock of the semantics instead.
package koinetest_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/station"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/manifest"
	koinetest "github.com/sol-duara-inc/koine-go/koine/testing"
)

// lastGood is the record's answer to history.last: the deployment that
// succeeded before this one failed. It is a domain object, because that is
// all a fulfiller ever answers with.
var lastGood = deployment.SeedDeploymentFinished("payments-api", koine.Success, "sha256:good", "prod")

// TestHarness_DrivesTheStewardWithAScriptedExchange is §9's example, run.
// The station's failure branch consumes history.last inline and speaks the
// deploy that follows from the answer.
func TestHarness_DrivesTheStewardWithAScriptedExchange(t *testing.T) {
	out := koinetest.Run(station.DeploymentSteward{},
		koinetest.Deliver(deployment.ResolvedDelivery{
			Subject:     "payments-api",
			Outcome:     koine.Failure,
			ArtifactID:  "sha256:bad",
			Environment: "prod",
		}),
		koinetest.Exchange("history.last", lastGood))

	if out.Identity.String() != "payment-engineering/mchen/deployment-steward" {
		t.Errorf("identity = %s", out.Identity)
	}
	if len(out.Awaits) != 1 || out.Awaits[0].Type != "dev.cdevents.deployment" || out.Awaits[0].Anchor != "deploy" {
		t.Errorf("awaits = %#v", out.Awaits)
	}
	if out.Complete != koine.DefaultAllAwaited {
		t.Errorf("complete = %#v", out.Complete)
	}

	if len(out.Utterances) != 1 {
		t.Fatalf("the failure branch spoke %d utterances, want one: %#v", len(out.Utterances), out.Utterances)
	}
	spoke, ok := out.Utterances[0].(deployment.Deploy)
	if !ok {
		t.Fatalf("the failure branch spoke %#v", out.Utterances[0])
	}
	if spoke.Artifact != "sha256:good" {
		t.Errorf("the deploy did not carry the last good artifact: %q", spoke.Artifact)
	}
	if spoke.Target != "prod" {
		t.Errorf("the deploy targeted %q", spoke.Target)
	}

	if len(out.Exchanges) != 1 {
		t.Fatalf("the run spoke %d exchanges, want one", len(out.Exchanges))
	}
	ex := out.Exchanges[0]
	if ex.Name != "history.last" || ex.Seat != "history" {
		t.Errorf("exchange = %#v", ex)
	}
	if len(ex.Args) != 1 || ex.Args[0].Name != "outcome" || ex.Args[0].Value != string(koine.Success) {
		t.Errorf("the intent carried %#v", ex.Args)
	}
	if ex.Consumption != manifest.Inline {
		t.Errorf("Value() was consumed; the run saw %q", ex.Consumption)
	}
	if out.Consumption["history.last"] != manifest.Inline {
		t.Errorf("consumption = %q, want inline", out.Consumption["history.last"])
	}
}

// TestHarness_TheSuccessBranchSpeaksWithoutSpeakingToAnyone pins that the
// harness observes rather than drives: the success branch needs no exchange,
// and scripting one does not conjure a call.
func TestHarness_TheSuccessBranchSpeaksWithoutSpeakingToAnyone(t *testing.T) {
	out := koinetest.Run(station.DeploymentSteward{},
		koinetest.Deliver(deployment.ResolvedDelivery{
			Outcome:     koine.Success,
			ArtifactID:  "sha256:fine",
			Environment: "prod",
		}),
		koinetest.Exchange("history.last", lastGood))

	if len(out.Exchanges) != 0 {
		t.Fatalf("the success branch spoke %#v", out.Exchanges)
	}
	if len(out.Utterances) != 1 {
		t.Fatalf("the success branch spoke %d utterances", len(out.Utterances))
	}
	rec, ok := out.Utterances[0].(deployment.DeploymentRecorded)
	if !ok || rec.Artifact != "sha256:fine" {
		t.Fatalf("the success branch spoke %#v", out.Utterances[0])
	}
}

// TestHarness_TheTypedVariantIsBranchedOnNotDefendedAgainst pins E-C's
// posture: the error a handle returns is the typed outcome variant of the
// expected response, and the body branches on it like any other future.
func TestHarness_TheTypedVariantIsBranchedOnNotDefendedAgainst(t *testing.T) {
	nothingGood := errors.New("no deployment of this subject has ever succeeded")
	out := koinetest.Run(station.DeploymentSteward{},
		koinetest.Deliver(deployment.ResolvedDelivery{
			Outcome:     koine.Failure,
			ArtifactID:  "sha256:bad",
			Environment: "prod",
		}),
		koinetest.ExchangeFails("history.last", nothingGood))

	if len(out.Exchanges) != 1 || !errors.Is(out.Exchanges[0].Err, nothingGood) {
		t.Fatalf("the run did not carry the variant: %#v", out.Exchanges)
	}
	if len(out.Utterances) != 1 {
		t.Fatalf("the variant branch spoke %d utterances", len(out.Utterances))
	}
	if _, ok := out.Utterances[0].(deployment.DeploymentRecorded); !ok {
		t.Fatalf("the variant branch spoke %#v", out.Utterances[0])
	}
	if out.Consumption["history.last"] != manifest.Inline {
		t.Errorf("a consumed variant is still inline, got %q", out.Consumption["history.last"])
	}
}

// TestHarness_BothConsumptionPatternsAreWitnessed pins that a run can tell
// the two chain roles a station's own consumption can produce apart — which
// is what lets the conformance gate compare the manifest word for word
// instead of through a collapse. A third pattern, naming a workflow-owned
// topology fact rather than anything a station author waits on, lived here
// through Draft 2 and was struck 2026-08-27 (DESIGN.md §6, issue #11).
func TestHarness_BothConsumptionPatternsAreWitnessed(t *testing.T) {
	out := koinetest.Run(station.DeploymentAuditor{},
		koinetest.Deliver(deployment.ResolvedDelivery{
			Outcome:     koine.Failure,
			ArtifactID:  "sha256:bad",
			Environment: "prod",
		}),
		koinetest.Exchange("history.last", lastGood))

	if len(out.Exchanges) != 1 {
		t.Fatalf("the auditor spoke %d exchanges, want one", len(out.Exchanges))
	}
	if out.Consumption["history.last"] != manifest.Concurrent {
		t.Errorf("spoken, never consumed — the gate stands: observed as %q", out.Consumption["history.last"])
	}
	if out.Exchanges[0].Consumption != manifest.Concurrent {
		t.Errorf("history.last: Spoken says %q, want %q", out.Exchanges[0].Consumption, manifest.Concurrent)
	}

	// And the steward's inline reading is still its own thing, so the two
	// roles are two and not one wearing two names.
	inline := koinetest.Run(station.DeploymentSteward{},
		koinetest.Deliver(deployment.ResolvedDelivery{Outcome: koine.Failure, ArtifactID: "sha256:bad"}),
		koinetest.Exchange("history.last", lastGood))
	if inline.Consumption["history.last"] != manifest.Inline {
		t.Errorf("the steward's consumed exchange observed as %q", inline.Consumption["history.last"])
	}
}

// TestHarness_AnUnscriptedExchangeIsLoud pins the harness's own refusal
// posture: it never invents an answer, because a run that did would be
// asserting on a fact the author never stated.
func TestHarness_AnUnscriptedExchangeIsLoud(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("an unscripted exchange was answered")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "history.last") || !strings.Contains(msg, "nothing scripted") {
			t.Fatalf("panic said %v", r)
		}
	}()
	koinetest.Run(station.DeploymentSteward{},
		koinetest.Deliver(deployment.ResolvedDelivery{Outcome: koine.Failure}))
}

// TestHarness_ARefusedYieldStopsResolution pins the Yield contract: false
// means the host cancelled, and nothing after the refusal is spoken.
func TestHarness_ARefusedYieldStopsResolution(t *testing.T) {
	out := koinetest.Run(station.DeploymentAuditor{},
		koinetest.Deliver(deployment.ResolvedDelivery{Outcome: koine.Failure, ArtifactID: "sha256:bad"}),
		koinetest.Exchange("history.last", lastGood),
		koinetest.StopAfter(0))

	if len(out.Utterances) != 0 {
		t.Fatalf("a refused yield still stored %#v", out.Utterances)
	}
}

// TestHarness_RunNeedsADelivery pins that a station is a function OF a
// delivery: without one there is nothing for the run to be a function of,
// and the harness says so rather than resolving a zero value.
func TestHarness_RunNeedsADelivery(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Run resolved a station with no delivery")
		}
	}()
	koinetest.Run(station.DeploymentSteward{})
}
