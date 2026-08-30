package wire_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/station"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
	"github.com/sol-duara-inc/koine-go/koine/wire"
)

// passUpStation wraps a pass-up fixture for the guest runtime.
func passUpStation(k koine.Koine) wire.Station {
	return wire.Station{
		Koine:    k,
		Manifest: []byte(`{"identity":{"name":"chain"}}`),
		Decode: func(f wire.DeliveryFrame) (koine.Delivery, error) {
			var d deployment.ResolvedDelivery
			if err := d.UnmarshalJSON(f.Event); err != nil {
				return nil, err
			}
			return d, nil
		},
	}
}

const succeededDeployment = `{"subject":"payments-api","outcome":"success",` +
	`"artifactId":"sha256:fine","environment":"prod"}`

// TestWire_VerbFormAndHookFormProduceTheSameFrames is koine-go#5's second
// done-condition, held to its own sentence: the named hooks are SUGAR over
// the three verbs, so the BYTES that leave the guest must be the same bytes.
//
// Comparing harness records would compare this repository's bookkeeping.
// What the parent receives is a frame, so a frame is what is compared —
// encoded, byte for byte, as it left.
func TestWire_VerbFormAndHookFormProduceTheSameFrames(t *testing.T) {
	cases := []struct {
		name  string
		event string
	}{
		{"a failed deployment", failedDeployment},
		{"a succeeded deployment", succeededDeployment},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			verbs := &scriptHost{answer: wire.AnswerFrame{Status: 200}}
			if got := wire.New(passUpStation(&station.ChainVerbs{}), verbs).
				Deliver(stewardDelivery(c.event)); got != wire.Resolved {
				t.Fatalf("verb form ended %s", got)
			}
			hooks := &scriptHost{answer: wire.AnswerFrame{Status: 200}}
			if got := wire.New(passUpStation(&station.ChainHooks{}), hooks).
				Deliver(stewardDelivery(c.event)); got != wire.Resolved {
				t.Fatalf("hook form ended %s", got)
			}

			if len(verbs.rawExchanges) != len(hooks.rawExchanges) {
				t.Fatalf("verb form sent %d frames, hook form %d",
					len(verbs.rawExchanges), len(hooks.rawExchanges))
			}
			if len(verbs.rawExchanges) != 1 {
				t.Fatalf("each form makes one passage; got %d", len(verbs.rawExchanges))
			}
			for i := range verbs.rawExchanges {
				if string(verbs.rawExchanges[i]) != string(hooks.rawExchanges[i]) {
					t.Fatalf("frame %d differs:\n verbs %s\n hooks %s",
						i, verbs.rawExchanges[i], hooks.rawExchanges[i])
				}
			}
			// And both asked for the answer — declaring Post is
			// declaring the await, which is why the hook form has no
			// Await written anywhere.
			if verbs.valPolls != 1 || hooks.valPolls != 1 {
				t.Errorf("verbs polled %d, hooks polled %d; both ask once",
					verbs.valPolls, hooks.valPolls)
			}
		})
	}
}

// TestWire_OnePassagePerDeliveryAndWithholdIsTheOnlyGate pins the three
// cases the state machine admits. Without it, a station that declares Pre
// AND calls the verbs passes twice, and a body that Withheld finds the hook
// passing anyway — the only suppression failing to suppress.
func TestWire_OnePassagePerDeliveryAndWithholdIsTheOnlyGate(t *testing.T) {
	t.Run("withhold first, then the hook's pass is a no-op", func(t *testing.T) {
		host := &scriptHost{answer: wire.AnswerFrame{Status: 200}}
		if got := wire.New(passUpStation(&withholdThenHook{}), host).
			Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
			t.Fatalf("deliver = %s", got)
		}
		if len(host.exchanges) != 1 {
			t.Fatalf("the station spoke %d frames, want one withhold: %#v", len(host.exchanges), host.exchanges)
		}
		if host.exchanges[0].Type != wire.TypeWithhold {
			t.Fatalf("the surviving frame is %q — the hook's pass beat the withhold", host.exchanges[0].Type)
		}
	})

	t.Run("a second pass is refused by name", func(t *testing.T) {
		defer expectTrap(t, "passed up twice")
		wire.New(passUpStation(&passesTwice{}), &scriptHost{answer: wire.AnswerFrame{Status: 200}}).
			Deliver(stewardDelivery(failedDeployment))
	})

	t.Run("a withhold after a pass is refused by name", func(t *testing.T) {
		defer expectTrap(t, "already passed up")
		wire.New(passUpStation(&withholdsTooLate{}), &scriptHost{answer: wire.AnswerFrame{Status: 200}}).
			Deliver(stewardDelivery(failedDeployment))
	})
}

func expectTrap(t *testing.T, wantIn string) {
	t.Helper()
	r := recover()
	if r == nil {
		t.Fatal("the station carried on")
	}
	msg, ok := r.(string)
	if !ok || !strings.Contains(msg, wantIn) {
		t.Fatalf("the trap said %v", r)
	}
}

// withholdThenHook withholds in its body AND declares a Pre hook. The hook
// would pass up at step end; the withhold is what stops it.
type withholdThenHook struct{ koine.ObserverBase }

func (withholdThenHook) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "n"}
}
func (withholdThenHook) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (withholdThenHook) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (s *withholdThenHook) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	s.Withhold(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}
func (withholdThenHook) Pre(d koine.Delivery) koine.Utterance {
	dep := d.(deployment.ResolvedDelivery)
	return deployment.DeploymentRecorded{Artifact: dep.ArtifactID}
}

// passesTwice hands the parent two different objects in one delivery.
type passesTwice struct{ koine.ObserverBase }

func (passesTwice) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "n"}
}
func (passesTwice) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (passesTwice) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (s *passesTwice) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	s.PassUp(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
	s.PassUp(deployment.Deploy{Artifact: dep.ArtifactID, Target: dep.Environment})
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}

// withholdsTooLate tries to unsay what the parent already has.
type withholdsTooLate struct{ koine.ObserverBase }

func (withholdsTooLate) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "n"}
}
func (withholdsTooLate) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (withholdsTooLate) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (s *withholdsTooLate) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	s.PassUp(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
	s.Withhold(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}

// TestWire_AnUnfilledParentIsDeterminateNotAStoppage makes the unfilled-seat
// branch live. It is recognised by the engine's own ErrNotImplemented text,
// and it is NOT a stoppage: nothing malfunctioned, the seat is empty, and the
// same child starts working the day the parent installs.
func TestWire_AnUnfilledParentIsDeterminateNotAStoppage(t *testing.T) {
	host := &scriptHost{answer: wire.AnswerFrame{
		Status: 501,
		Error:  wire.UnfilledPrefix + `: no controller serves "com.example.payments-engineering"`,
	}}
	guest := wire.New(passUpStation(&unfilledWatcher{}), host)
	if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("an unfilled parent stopped the run: %s (%s)", got, guest.Fault())
	}
	if guest.Fault() != "" {
		t.Errorf("an empty seat was recorded as a stoppage below the line: %s", guest.Fault())
	}
	if len(host.yields) != 1 {
		t.Fatalf("the body spoke %d utterances", len(host.yields))
	}
	if got := host.yields[0]["artifact"]; !strings.Contains(got.(string), "no controller serves") {
		t.Errorf("the body was handed %v", got)
	}
	// The prefix is the engine's own sentinel text, not this SDK's guess.
	if wire.UnfilledPrefix != "koine: function not implemented" {
		t.Errorf("UnfilledPrefix = %q; koine.ErrNotImplemented reads \"koine: function not implemented\"", wire.UnfilledPrefix)
	}
}

// unfilledWatcher speaks whatever its parent's absence was called.
type unfilledWatcher struct{ koine.ObserverBase }

func (unfilledWatcher) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "n"}
}
func (unfilledWatcher) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (unfilledWatcher) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (s *unfilledWatcher) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	got := s.Await(s.PassUp(deployment.DeploymentRecorded{Artifact: dep.ArtifactID}))
	var unfilled *wire.Unfilled
	if errors.As(got.Err, &unfilled) {
		yield(deployment.DeploymentRecorded{Artifact: deployment.ArtifactRef(unfilled.Why)})
		return
	}
	yield(deployment.DeploymentRecorded{Artifact: "filled"})
}
