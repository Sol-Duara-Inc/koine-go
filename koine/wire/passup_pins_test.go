package wire_test

// PINS FOR THE PASS-UP, ON THE WIRE (GH-18). Each exists because a deliberate
// mutation of the SDK passed the whole suite.
//
// The recurring shape: the EXCHANGE path's stoppage handling is thoroughly
// pinned (TestWire_AStoppageBelowTheLineIsNotTheAuthorsTrap), and the PASS-UP
// path — built the same way, shipping into the same sandbox — is not. Two
// paths written to one rule, tested differently.

import (
	"errors"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
	"github.com/sol-duara-inc/koine-go/koine/wire"
)

// ---- fixtures ----

// passesAndSpeaks hands the parent an object and then speaks, WITHOUT
// awaiting. The yield after the pass is the point: if the pass silently
// failed, that yield must not be stored.
type passesAndSpeaks struct{ koine.ObserverBase }

func (passesAndSpeaks) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "n"}
}
func (passesAndSpeaks) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (passesAndSpeaks) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (s *passesAndSpeaks) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	s.PassUp(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}

// withholdsAndSpeaks suppresses the default pass and then speaks.
type withholdsAndSpeaks struct{ koine.ObserverBase }

func (withholdsAndSpeaks) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "n"}
}
func (withholdsAndSpeaks) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (withholdsAndSpeaks) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (s *withholdsAndSpeaks) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	s.Withhold(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}

// withholdsThenAwaits suppresses its pass and then asks what its parent
// concluded about it — a station branching on its own suppression.
type withholdsThenAwaits struct{ koine.ObserverBase }

func (withholdsThenAwaits) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "n"}
}
func (withholdsThenAwaits) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (withholdsThenAwaits) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (s *withholdsThenAwaits) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	s.Withhold(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
	got := s.Await(0)
	name := "unexpected"
	switch {
	case errors.Is(got.Err, koine.ErrAwaitedWithheld) && got.Outcome == koine.Success:
		name = "withheld"
	case got.Outcome == koine.Failure:
		name = "failure"
	}
	yield(deployment.DeploymentRecorded{Artifact: deployment.ArtifactRef(name)})
}

// postWithoutPre declares Post and no Pre — the shape koinegen refuses
// statically and RunHooks refuses at runtime.
type postWithoutPre struct{ koine.ObserverBase }

func (postWithoutPre) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "n"}
}
func (postWithoutPre) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (postWithoutPre) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (s *postWithoutPre) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}
func (postWithoutPre) Post(koine.Utterance, koine.Conclusion) {}

// decliningPre answers nothing from Pre: a station declining to hand
// anything over. NOT a withhold — simply nothing to pass.
type decliningPre struct{ koine.ObserverBase }

func (decliningPre) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "n"}
}
func (decliningPre) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (decliningPre) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (s *decliningPre) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}
func (decliningPre) Pre(koine.Delivery) koine.Utterance { return nil }

// ---- the pins ----

// A HOST THAT WILL NOT TAKE THE PASS-UP IS A STOPPAGE. The exchange path's
// identical guard is pinned; this one was not, and the consequence is the
// worst kind: SILENCE THAT REPORTS SUCCESS.
//
// THE MUTATIONS: delete the `if handle == 0` guard in PassUp, and separately
// the one in Withhold.
//
// Without the PassUp guard no fault is recorded, so the emit gate never
// closes, so every utterance the body speaks afterwards IS stored and the run
// ends Resolved — while the parent never received anything. That is exactly
// what the package doc forbids: "nothing after a stoppage is stored, so a
// body cannot turn one into an event naming something that never happened."
//
// Without the Withhold guard a refused suppression is ignored, and because
// THE DEFAULT PASS IS THE HOST'S, the parent then receives the very object
// the author explicitly suppressed — the one suppression there is, failing
// silently at the single point where the guest depends on being heard.
func TestWire_AHostRefusingThePassUpPathIsAStoppage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		station func() koine.Koine
		wantIn  string
	}{
		{"the host would not take the pass-up", func() koine.Koine { return &passesAndSpeaks{} },
			"would not take a pass-up"},
		{"the host would not take the withhold", func() koine.Koine { return &withholdsAndSpeaks{} },
			"would not take a withhold"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The control FIRST: with a host that answers, this station
			// resolves and its speech IS stored.
			ok := &scriptHost{answer: wire.AnswerFrame{Status: 200}}
			okGuest := wire.New(passUpStation(tc.station()), ok)
			if got := okGuest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
				t.Fatalf("precondition: a working host must resolve, got %s (%s)", got, okGuest.Fault())
			}
			if len(ok.yields) != 1 {
				t.Fatalf("precondition: the body must speak once, got %d", len(ok.yields))
			}

			host := &scriptHost{noHandle: true}
			guest := wire.New(passUpStation(tc.station()), host)
			got := guest.Deliver(stewardDelivery(failedDeployment))

			if got != wire.Unanswered {
				t.Fatalf("deliver = %s, want unanswered — a host refusing the pass-up path is a "+
					"stoppage below the line, and reporting success for it means the parent never "+
					"got the object while the run says it did", got)
			}
			if got.AttributedToAuthor() {
				t.Error("a stoppage below the line was attributed to the author")
			}
			if !strings.Contains(guest.Fault(), tc.wantIn) {
				t.Fatalf("the fault did not say %q: %q", tc.wantIn, guest.Fault())
			}
			// Spoken once through the host's own diagnostic channel, marked.
			var marked []string
			for _, line := range host.logs {
				if strings.HasPrefix(line, wire.FaultPrefix) {
					marked = append(marked, line)
				}
			}
			if len(marked) != 1 {
				t.Fatalf("the host heard %d attributed lines, want one: %v", len(marked), host.logs)
			}
			// And the gate is CLOSED: nothing the body said afterwards is stored.
			if len(host.yields) != 0 {
				t.Fatalf("a stoppage below the line still stored %#v — a body must not be able to "+
					"turn one into an event naming something that never happened", host.yields)
			}
		})
	}
}

// AWAITING YOUR OWN WITHHELD PASS IS THE SHARED ANSWER, ON THE WIRE TOO.
//
// THE MUTATION: delete the `if c, ok := l.g.passing.AwaitedConclusion(); ok`
// consultation in AwaitPass. A station that withholds and then awaits is then
// handed Failure plus a *Stopped instead of Success plus
// koine.ErrAwaitedWithheld — so `errors.Is(err, koine.ErrAwaitedWithheld)`
// stops matching, a body branching on its own suppression takes the wrong
// branch, and Conclusion.OK() flips.
//
// THIS DRIFT WAS REAL ONCE. koine/testing/passup_test.go records it verbatim:
// "the wire answered Success plus the shared finding while the bench answered
// the parent's scripted conclusion for a pass that never left." The regression
// pin was placed on the BENCH only — and the wire is the half that ships into
// the sandbox.
func TestWire_AwaitOfAWithheldPassIsTheSharedAnswer(t *testing.T) {
	host := &scriptHost{answer: wire.AnswerFrame{Status: 200}}
	guest := wire.New(passUpStation(&withholdsThenAwaits{}), host)
	if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("deliver = %s (%s)", got, guest.Fault())
	}
	if len(host.yields) != 1 {
		t.Fatalf("the body spoke %d utterances, want one", len(host.yields))
	}
	if got := host.yields[0]["artifact"]; got != "withheld" {
		t.Fatalf("the body was handed %v, want the withheld answer — awaiting a suppressed pass "+
			"must be Success with koine.ErrAwaitedWithheld, not a Failure with a *Stopped", got)
	}
	// The withhold itself still left, because the default pass is the host's.
	if len(host.exchanges) != 1 || host.exchanges[0].Type != wire.TypeWithhold {
		t.Fatalf("the guest spoke %#v, want exactly one withhold", host.exchanges)
	}
}

// POST WITHOUT PRE IS REFUSED BY NAME, AT RUNTIME AS WELL AS AT GENERATION.
// The rule has TWO independent implementations — koinegen's static refusal
// and RunHooks' runtime one — and only the generator's was tested.
// TestManifest_PostWithoutPreIsRefusedByName reads like the pin for this rule
// and never calls RunHooks.
//
// THE MUTATION: `if !hasPre { return ErrPostWithoutPre }` → `return nil`.
// Any station reaching RunHooks without passing koinegen — a hand-assembled
// wire.Station, a Post promoted through embedding after generation, a guest
// from another toolchain — then has its Post silently skipped: it asked what
// its parent concluded and is told nothing, with no error and no trap.
func TestRunHooks_PostWithoutPreIsRefusedByName(t *testing.T) {
	t.Run("RunHooks answers the named error", func(t *testing.T) {
		var d deployment.ResolvedDelivery
		if err := d.UnmarshalJSON([]byte(failedDeployment)); err != nil {
			t.Fatal(err)
		}
		l := &stubLineage{}
		err := koine.RunHooks(&postWithoutPre{}, d, l)
		if !errors.Is(err, koine.ErrPostWithoutPre) {
			t.Fatalf("RunHooks = %v, want koine.ErrPostWithoutPre — the runtime guard is the only "+
				"one a station that never passed through koinegen ever meets", err)
		}
		// And it refuses BEFORE touching the parent: a refused shape must
		// not have half-spoken.
		if l.passed != 0 || l.withheld != 0 {
			t.Errorf("the refused station still reached its parent (passed=%d withheld=%d)",
				l.passed, l.withheld)
		}
	})

	t.Run("and it traps through the wire host", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("a Post-without-Pre station ran to completion; its Post was silently skipped")
			}
			msg, _ := r.(string)
			if !strings.Contains(msg, "Post without Pre") {
				t.Fatalf("the trap said %v; it must name the pair so the author can see which "+
					"half is missing", r)
			}
		}()
		wire.New(passUpStation(&postWithoutPre{}), &scriptHost{answer: wire.AnswerFrame{Status: 200}}).
			Deliver(stewardDelivery(failedDeployment))
	})
}

// A DECLINING Pre PASSES NOTHING AT ALL. The contract is stated in RunHooks
// itself: "Pre answering nothing is a station declining to hand anything
// over... It is NOT a Withhold... this is simply nothing to pass."
//
// THE MUTATION: delete the `if offered == nil { return nil }` guard. A
// declining Pre then sends a real koine.passup exchange carrying no Intent at
// all, AND consumes the one-passage-per-delivery budget through Offer() — so
// any later explicit PassUp in the same delivery TRAPS as "passed up twice"
// for a pass the author never made. A decline becomes an empty pass plus a
// booby-trapped budget.
func TestRunHooks_ADecliningPrePassesNothing(t *testing.T) {
	host := &scriptHost{answer: wire.AnswerFrame{Status: 200}}
	guest := wire.New(passUpStation(&decliningPre{}), host)
	if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("deliver = %s (%s)", got, guest.Fault())
	}
	if len(host.exchanges) != 0 {
		t.Fatalf("a declining Pre spoke %#v — declining is not withholding and not passing; it "+
			"is nothing said at all, and an empty frame both misinforms the parent and spends "+
			"the one passage this delivery had", host.exchanges)
	}
	// The body still ran and was stored: declining concerns the parent, not
	// the record.
	if len(host.yields) != 1 {
		t.Fatalf("the body spoke %d utterances, want one", len(host.yields))
	}
}

// THE UNFILLED SENTINEL IS THE ENGINE'S TEXT, and this test can only prove
// half of that from inside the SDK.
//
// TestWire_AnUnfilledParentIsDeterminateNotAStoppage builds its host answer
// out of wire.UnfilledPrefix and then matches with the same constant, so the
// behavioural half round-trips the symbol against itself and passes however
// far the constant drifts from the engine. PROVEN: drifting UnfilledPrefix by
// one character left the whole module green.
//
// The literal self-check below is a genuine pin but a local one: it compares
// the SDK to a string typed in the SDK's own test, so it catches someone
// editing the constant and never the ENGINE changing its text underneath.
// The other half — comparing against conduit-go's exported
// koine.ErrNotImplemented — belongs in the conformance module, where
// conduit-go is importable, and is added there.
func TestWire_TheUnfilledPrefixIsTheEnginesOwnSentence(t *testing.T) {
	const engineText = "koine: function not implemented"
	if wire.UnfilledPrefix != engineText {
		t.Errorf("UnfilledPrefix = %q, want %q (conduit-go pkg/koine.ErrNotImplemented). A child "+
			"of a declared-but-not-loaded parent stops matching the day these diverge: isUnfilled "+
			"fails, the answer falls to the Stoppage branch, and the run ends Unanswered.",
			wire.UnfilledPrefix, engineText)
	}
	// A drifted constant must not still satisfy the behavioural test. Assert
	// the RECOGNISER against a literal engine sentence rather than against
	// the constant, so this cannot round-trip.
	host := &scriptHost{answer: wire.AnswerFrame{
		Status: 501,
		Error:  engineText + `: no controller serves "com.example.payments-engineering"`,
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
	if got, _ := host.yields[0]["artifact"].(string); !strings.Contains(got, "no controller serves") {
		t.Errorf("the body was handed %v", host.yields[0]["artifact"])
	}
}

// stubLineage lets RunHooks be called directly, with no host and no
// transport — the point being that the runtime guard fires before either is
// touched, which is what `passed`/`withheld` staying zero proves.
type stubLineage struct{ passed, withheld int }

func (l *stubLineage) PassUp(koine.Utterance) koine.Passage { l.passed++; return 0 }
func (l *stubLineage) Withhold(koine.Utterance)             { l.withheld++ }
func (l *stubLineage) AwaitPass(koine.Passage) koine.Conclusion {
	return koine.Conclusion{Outcome: koine.Success}
}
