package koinetest_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/station"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
	koinetest "github.com/sol-duara-inc/koine-go/koine/testing"
)

func failed() deployment.ResolvedDelivery {
	return deployment.ResolvedDelivery{
		Subject: "payments-api", Outcome: koine.Failure,
		ArtifactID: "sha256:bad", Environment: "prod",
	}
}

// TestHarness_VerbFormAndHookFormHandTheParentTheSameThing is the ticket's
// second done-condition: the named hooks are SUGAR over the three verbs and
// never a second mechanism, so the two forms must be indistinguishable from
// the parent's side.
//
// What is compared is the traffic, not the source. The twins differ in where
// the pass-up sits — the verb form puts it where the author wrote the line,
// the hook form at step end, which is the default position — and that
// difference is the substrate rather than a discrepancy.
func TestHarness_VerbFormAndHookFormHandTheParentTheSameThing(t *testing.T) {
	verbs := koinetest.Run(&station.ChainVerbs{}, koinetest.Deliver(failed()))
	hooks := koinetest.Run(&station.ChainHooks{}, koinetest.Deliver(failed()))

	if len(verbs.Passages) != 1 || len(hooks.Passages) != 1 {
		t.Fatalf("verbs made %d passages, hooks made %d; each should make one",
			len(verbs.Passages), len(hooks.Passages))
	}
	v, h := verbs.Passages[0], hooks.Passages[0]

	if v.Withheld || h.Withheld {
		t.Error("neither twin withholds")
	}
	if !v.Awaited || !h.Awaited {
		t.Errorf("both twins ask for the answer: verbs=%v hooks=%v", v.Awaited, h.Awaited)
	}

	// The object itself, compared as the parent would receive it.
	want, err := v.Offered.(interface{ MarshalJSON() ([]byte, error) }).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	got, err := h.Offered.(interface{ MarshalJSON() ([]byte, error) }).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("the twins hand up different objects:\n verbs %s\n hooks %s", want, got)
	}
	if v.Offered.(interface{ EventType() string }).EventType() !=
		h.Offered.(interface{ EventType() string }).EventType() {
		t.Error("the twins hand up different event types")
	}
}

// TestHarness_ZeroCodeMakesNoPassageOfItsOwn is the third done-condition. A
// station that writes no verb and declares no hook still passes up — but the
// step-end default pass is the HOST's duty, so the guest surface must emit
// nothing of its own. Silence here is the assertion.
func TestHarness_ZeroCodeMakesNoPassageOfItsOwn(t *testing.T) {
	for name, k := range map[string]koine.Koine{
		"the steward":   &station.DeploymentSteward{},
		"the auditor":   &station.DeploymentAuditor{},
		"the rehearsal": &station.DeploymentRehearsal{},
	} {
		t.Run(name, func(t *testing.T) {
			out := koinetest.Run(k,
				koinetest.Deliver(failed()),
				koinetest.Exchange("history.last",
					deployment.SeedDeploymentFinished("payments-api", koine.Success, "sha256:good", "prod")),
				koinetest.Exchange("ledger.note", deployment.SeedDeploymentRecorded("sha256:bad")))
			if len(out.Passages) != 0 {
				t.Fatalf("a station that wrote nothing made %d passages: %#v", len(out.Passages), out.Passages)
			}
		})
	}
}

// TestHarness_WithholdIsTheOnlySuppression is the fourth done-condition.
// Withholding is written down and visible; nothing else suppresses.
func TestHarness_WithholdIsTheOnlySuppression(t *testing.T) {
	out := koinetest.Run(&station.ChainWithholds{}, koinetest.Deliver(failed()))
	if len(out.Passages) != 1 {
		t.Fatalf("the station withheld once; the run saw %d", len(out.Passages))
	}
	if !out.Passages[0].Withheld {
		t.Error("a withhold was recorded as a pass")
	}
	if out.Passages[0].Awaited {
		t.Error("a withheld pass has no answer to wait for")
	}
	// It still speaks its own utterance: withholding the pass is not
	// withholding the station's own speech.
	if len(out.Utterances) != 1 {
		t.Fatalf("the station spoke %d utterances", len(out.Utterances))
	}
}

// TestHarness_AParentFindingArrivesAsAValue is the fifth done-condition. The
// parent's outcome reaches the child as a VALUE — no panic, no unwind —
// because asking for it is declaring that it will be handled.
func TestHarness_AParentFindingArrivesAsAValue(t *testing.T) {
	breach := errors.New("the parent's tool would not answer")

	out := koinetest.Run(&station.ChainVerbs{},
		koinetest.Deliver(failed()),
		koinetest.Parent(koine.Conclusion{Outcome: koine.Failure, Err: breach}))

	if len(out.Utterances) != 1 {
		t.Fatalf("the finding branch spoke %d utterances", len(out.Utterances))
	}
	if _, ok := out.Utterances[0].(deployment.Deploy); !ok {
		t.Fatalf("the body did not take its finding branch: %#v", out.Utterances[0])
	}

	// And the plain case is plain.
	plain := koinetest.Run(&station.ChainVerbs{}, koinetest.Deliver(failed()))
	if len(plain.Utterances) != 1 {
		t.Fatalf("the plain branch spoke %d utterances", len(plain.Utterances))
	}
	if _, ok := plain.Utterances[0].(deployment.DeploymentRecorded); !ok {
		t.Fatalf("the plain branch spoke %#v", plain.Utterances[0])
	}

	// A hook station is handed the same value.
	hooks := &station.ChainHooks{}
	koinetest.Run(hooks,
		koinetest.Deliver(failed()),
		koinetest.Parent(koine.Conclusion{Outcome: koine.Failure, Err: breach}))
	if !errors.Is(hooks.Concluded().Err, breach) {
		t.Fatalf("Post was told %#v", hooks.Concluded())
	}
	if hooks.Concluded().OK() {
		t.Error("a conclusion carrying a finding is not OK")
	}
}

// TestHarness_ConstructionIsDeliveryInTheHarnessToo pins what the harness had
// been quietly skipping: it builds the station the way a host does, so a body
// that reads its chain or speaks to its parent finds what it would find in
// the engine. A harness that skipped construction would be testing a station
// nobody built.
func TestHarness_ConstructionIsDeliveryInTheHarnessToo(t *testing.T) {
	steward := &station.DeploymentSteward{}
	koinetest.Run(steward,
		koinetest.Standing("chain-9", "sub:mchen;act:conduit"),
		koinetest.Deliver(failed()),
		koinetest.Exchange("history.last",
			deployment.SeedDeploymentFinished("payments-api", koine.Success, "sha256:good", "prod")))

	if steward.Chain() != "chain-9" {
		t.Errorf("Chain() = %q", steward.Chain())
	}
	if steward.Actor() != "sub:mchen;act:conduit" {
		t.Errorf("Actor() = %q", steward.Actor())
	}
}

// TestHarness_AStationMustBeAddressable pins the other half of that: a
// station passed by value is one the host could not construct either, and the
// refusal says what to do about it.
func TestHarness_AStationMustBeAddressable(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("a station the host could not construct was accepted")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "Take its address") {
			t.Fatalf("the refusal said %v", r)
		}
	}()
	koinetest.Run(station.DeploymentSteward{}, koinetest.Deliver(failed()))
}

// TestHarness_OnePassagePerDeliveryHoldsOnTheBenchToo pins the same gate the
// sandbox enforces, from the author's own test surface. The bench holds a
// station to the rule the engine holds it to, or it is not the semantics
// minus the transport.
func TestHarness_OnePassagePerDeliveryHoldsOnTheBenchToo(t *testing.T) {
	t.Run("withhold suppresses a hook's pass", func(t *testing.T) {
		out := koinetest.Run(&withholdingHookStation{}, koinetest.Deliver(failed()))
		if len(out.Passages) != 1 {
			t.Fatalf("the station made %d passages: %#v", len(out.Passages), out.Passages)
		}
		if !out.Passages[0].Withheld {
			t.Fatal("the hook's pass beat the withhold — the only gate did not gate")
		}
	})

	t.Run("a second pass is refused by name", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("a station passed up twice and carried on")
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "passed up twice") {
				t.Fatalf("the trap said %v", r)
			}
		}()
		koinetest.Run(&doublePasser{}, koinetest.Deliver(failed()))
	})
}

// withholdingHookStation withholds in its body and declares a Pre hook that
// would otherwise pass at step end.
type withholdingHookStation struct{ koine.ObserverBase }

func (withholdingHookStation) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "n"}
}
func (withholdingHookStation) Awaits() []selector.Selector {
	return selector.List(deployment.Resolved())
}
func (withholdingHookStation) Complete() koine.Contract { return koine.DefaultAllAwaited }
func (s *withholdingHookStation) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	s.Withhold(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}
func (withholdingHookStation) Pre(d koine.Delivery) koine.Utterance {
	dep := d.(deployment.ResolvedDelivery)
	return deployment.DeploymentRecorded{Artifact: dep.ArtifactID}
}

// doublePasser hands the parent two different objects in one delivery.
type doublePasser struct{ koine.ObserverBase }

func (doublePasser) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "n"}
}
func (doublePasser) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (doublePasser) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (s *doublePasser) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	s.PassUp(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
	s.PassUp(deployment.Deploy{Artifact: dep.ArtifactID, Target: dep.Environment})
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}
