// These tests witness the generated strata from a FOREIGN package, the way
// author code sees them. Nothing here reaches inside the generator; every
// claim is made against the committed output.
package fixtures_test

import (
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/build"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/jenkins"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/payments"
	"github.com/sol-duara-inc/koine-go/koine"
)

// TestGenerate_StrataEmbedDownwardToTheFloor pins §5's shape: each stratum
// embeds the one below it, so the community agreement is carried rather than
// copied and the whole lineage is reachable from the deepest extension.
func TestGenerate_StrataEmbedDownwardToTheFloor(t *testing.T) {
	var p payments.PaymentsBuildFinished
	p.Subject = "payments-api" // floor key, promoted two strata up
	p.ExecutorNode = "agent-7" // vendor key
	p.CostCenter = "cc-1180"   // customer key
	p.Outcome = koine.Success  // floor key
	p.JenkinsBuildFinished.QueueWaitMS = 42

	if p.BuildFinished.Subject != "payments-api" {
		t.Fatalf("the floor's key did not land on the floor: %q", p.BuildFinished.Subject)
	}
	if p.JenkinsBuildFinished.ExecutorNode != "agent-7" {
		t.Fatalf("the vendor's key did not land on the vendor: %q", p.JenkinsBuildFinished.ExecutorNode)
	}
	if p.QueueWaitMS != 42 {
		t.Fatalf("promotion lost a vendor key: %d", p.QueueWaitMS)
	}

	// One uniform vocabulary: every stratum is speakable.
	var speakable []koine.Utterance = []koine.Utterance{
		build.BuildFinished{},
		jenkins.JenkinsBuildFinished{},
		payments.PaymentsBuildFinished{},
		deployment.Deploy{},
	}
	for _, u := range speakable {
		if u == nil {
			t.Fatal("a generated stratum type is not an Utterance")
		}
	}
}

// TestGenerate_EventTypeAndNamespaceTravelWithTheStratum pins that every
// generated type names the event it carries and the stratum it was declared
// at — the two facts the record stores as columns, not annotations.
func TestGenerate_EventTypeAndNamespaceTravelWithTheStratum(t *testing.T) {
	cases := []struct {
		got             interface{ EventType() string }
		event, declared string
		namespace       func() string
	}{
		{build.BuildFinished{}, "dev.cdevents.build.finished", "dev.cdevents.build", build.BuildFinished{}.Namespace},
		{jenkins.JenkinsBuildFinished{}, "dev.cdevents.build.finished", "io.jenkins", jenkins.JenkinsBuildFinished{}.Namespace},
		{payments.PaymentsBuildFinished{}, "dev.cdevents.build.finished", "com.example.payments-engineering", payments.PaymentsBuildFinished{}.Namespace},
	}
	for _, c := range cases {
		if c.got.EventType() != c.event {
			t.Errorf("EventType = %q, want %q", c.got.EventType(), c.event)
		}
		if c.namespace() != c.declared {
			t.Errorf("Namespace = %q, want %q", c.namespace(), c.declared)
		}
	}
}

// TestGenerate_SeedingIsWholeAtTheDeepestExtension pins the 2026-07-21
// ruling: objects are seeded whole at the deepest extension, and the seeder
// lives at the owning stratum. The customer's seeder takes the WHOLE object
// — community keys included — and honors the floor's agreement by calling
// the floor's own seeder rather than reassembling it.
func TestGenerate_SeedingIsWholeAtTheDeepestExtension(t *testing.T) {
	p := payments.SeedPaymentsBuildFinished(
		"payments-api", koine.Success, "sha256:abc", // the floor's keys
		"agent-7", 42, // the vendor's keys
		"cc-1180", true, // the customer's keys
	)
	switch {
	case p.Subject != "payments-api":
		t.Errorf("Subject = %q", p.Subject)
	case p.Outcome != koine.Success:
		t.Errorf("Outcome = %q", p.Outcome)
	case p.ArtifactID != "sha256:abc":
		t.Errorf("ArtifactID = %q", p.ArtifactID)
	case p.ExecutorNode != "agent-7":
		t.Errorf("ExecutorNode = %q", p.ExecutorNode)
	case p.QueueWaitMS != 42:
		t.Errorf("QueueWaitMS = %d", p.QueueWaitMS)
	case p.CostCenter != "cc-1180":
		t.Errorf("CostCenter = %q", p.CostCenter)
	case !p.PCIScope:
		t.Error("PCIScope did not survive seeding")
	}

	// The floor seeds only the floor; it has never heard of a vendor.
	f := build.SeedBuildFinished("payments-api", koine.Failure, "sha256:def")
	if f.Subject != "payments-api" || f.Outcome != koine.Failure {
		t.Fatalf("the floor's seeder built %#v", f)
	}
}

// TestGenerate_MarshalRoundTripsWithoutReflection pins the hand-rolled
// codec: every key is named in generated source, and a whole customer-stratum
// object survives the round trip.
func TestGenerate_MarshalRoundTripsWithoutReflection(t *testing.T) {
	want := payments.SeedPaymentsBuildFinished("payments-api", koine.Success, "sha256:abc", "agent-7", 42, "cc-1180", true)
	data, err := want.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	const wantJSON = `{"subject":"payments-api","outcome":"success","artifactId":"sha256:abc","executorNode":"agent-7","queueWaitMs":42,"costCenter":"cc-1180","pciScope":true}`
	if string(data) != wantJSON {
		t.Fatalf("marshal wrote\n  %s\nwant\n  %s", data, wantJSON)
	}
	var got payments.PaymentsBuildFinished
	if err := got.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round trip lost keys:\n got %#v\nwant %#v", got, want)
	}
}

// TestGenerate_ProjectedJSONCutsToLineage is the second of the two walls.
// The host projects before the guest sees bytes; even so, a floor type
// handed the whole customer-stratum document keeps the community keys and
// cannot hold the rest. Two walls, one semantic.
func TestGenerate_ProjectedJSONCutsToLineage(t *testing.T) {
	full := `{"subject":"payments-api","outcome":"success","artifactId":"sha256:abc",` +
		`"executorNode":"agent-7","queueWaitMs":42,"costCenter":"cc-1180","pciScope":true}`

	var floor build.BuildFinished
	if err := floor.UnmarshalJSON([]byte(full)); err != nil {
		t.Fatal(err)
	}
	if floor.Subject != "payments-api" || floor.Outcome != koine.Success || floor.ArtifactID != "sha256:abc" {
		t.Fatalf("the floor lost its own keys: %#v", floor)
	}
	back, err := floor.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, foreign := range []string{"executorNode", "costCenter", "pciScope", "queueWaitMs"} {
		if strings.Contains(string(back), foreign) {
			t.Errorf("a vendor or customer key survived a floor round trip: %s in %s", foreign, back)
		}
	}

	// The vendor stratum keeps its own and the floor's, and still drops the
	// customer's — downward blindness is not a filter anyone applied.
	var vendor jenkins.JenkinsBuildFinished
	if err := vendor.UnmarshalJSON([]byte(full)); err != nil {
		t.Fatal(err)
	}
	if vendor.ExecutorNode != "agent-7" || vendor.QueueWaitMS != 42 || vendor.Subject != "payments-api" {
		t.Fatalf("the vendor stratum lost keys: %#v", vendor)
	}
	back, err = vendor.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(back), "costCenter") {
		t.Errorf("a customer key survived a vendor round trip: %s", back)
	}
}

// TestGenerate_DeliveriesAreLiteralsAnAuthorCanWrite pins the shape §9's
// harness depends on: a delivery is a struct literal with the whole kingdom
// flat on it, so a test can state the facts and nothing else is required to
// construct one.
func TestGenerate_DeliveriesAreLiteralsAnAuthorCanWrite(t *testing.T) {
	d := deployment.ResolvedDelivery{
		Subject:     "payments-api",
		Outcome:     koine.Failure,
		ArtifactID:  "sha256:abc",
		Environment: "prod",
	}
	var asDelivery koine.Delivery = d
	if asDelivery == nil {
		t.Fatal("a generated delivery does not satisfy koine.Delivery")
	}
	if d.Projects() != "dev.cdevents.deployment.finished" || d.Namespace() != "dev.cdevents.deployment" {
		t.Fatalf("delivery %q/%q", d.Projects(), d.Namespace())
	}

	// The vendor delivery carries the vendor's keys; the floor's does not
	// have them at all, which is the compile-refusal fixture's whole point.
	v := jenkins.FinishedDelivery{Subject: "payments-api", ExecutorNode: "agent-7"}
	if v.ExecutorNode != "agent-7" {
		t.Fatalf("vendor delivery %#v", v)
	}
}

// TestGenerate_AnUnboundVerbIsLoud pins the posture every verb in this SDK
// takes: with no host below the station, speaking panics. A verb never fails
// silently and never answers with a zero value it invented.
func TestGenerate_AnUnboundVerbIsLoud(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("an unbound exchange answered quietly")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "history.last") || !strings.Contains(msg, "never fails silently") {
			t.Fatalf("panic said %v", r)
		}
	}()
	deployment.ResolvedDelivery{}.History().Last(koine.Success)
}
