package wire_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/station"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
	"github.com/sol-duara-inc/koine-go/koine/wire"
)

// scriptHost is a host written from a script: it records what the guest said
// and answers what the test told it to, in the host's own conventions. It is
// not a mock of the semantics — there are no semantics on this side of the
// boundary, only frames — which is why the guest runtime can be driven here
// at all. Whether these conventions ARE the host's is the conformance
// module's question, not this one's.
type scriptHost struct {
	yields    []map[string]any
	exchanges []wire.ExchangeFrame
	ackPolls  int
	valPolls  int
	logs      []string

	// what this host answers
	yieldCode  uint32 // the host's code; zero is success
	failYieldN int    // 1-based yield to fail; 0 fails none
	handle     uint64
	noHandle   bool
	acked      bool
	answer     wire.AnswerFrame
	emptyValue bool
	badValue   bool
}

func (h *scriptHost) Yield(frame []byte) uint32 {
	var spoke map[string]any
	if err := json.Unmarshal(frame, &spoke); err != nil {
		panic("the guest sent an unreadable yield frame: " + err.Error())
	}
	h.yields = append(h.yields, spoke)
	if h.failYieldN != 0 && len(h.yields) == h.failYieldN {
		return 1
	}
	return h.yieldCode
}

func (h *scriptHost) Exchange(frame []byte) uint64 {
	f, err := wire.DecodeExchange(frame)
	if err != nil {
		panic("the guest sent an unreadable exchange frame: " + err.Error())
	}
	h.exchanges = append(h.exchanges, f)
	if h.noHandle {
		return 0
	}
	if h.handle != 0 {
		return h.handle
	}
	return uint64(len(h.exchanges))
}

func (h *scriptHost) AckPoll(uint64) uint32 {
	h.ackPolls++
	if h.acked {
		return 1
	}
	return 0
}

func (h *scriptHost) ValuePoll(uint64) []byte {
	h.valPolls++
	switch {
	case h.emptyValue:
		return nil
	case h.badValue:
		return []byte(`{"status":`)
	}
	a := h.answer
	if a.Status == 0 && !a.Breach() && len(a.Value) == 0 {
		a.Status = 200
		a.Value = []byte(`{}`)
	}
	data, err := a.MarshalJSON()
	if err != nil {
		panic(err)
	}
	return data
}

func (h *scriptHost) Log(msg string) { h.logs = append(h.logs, msg) }

func stewardDelivery(event string) []byte {
	frame, err := wire.DeliveryFrame{
		Version:   wire.Version,
		Event:     []byte(event),
		EventType: "dev.cdevents.deployment.finished",
		Subject:   "payments-api",
		RunID:     "run-1",
		ChainID:   "chain-7",
		Actor:     "sub:mchen;act:conduit",
		Context:   map[string]string{wire.ContextAnchor: "deploy"},
	}.MarshalJSON()
	if err != nil {
		panic(err)
	}
	return frame
}

const failedDeployment = `{"subject":"payments-api","outcome":"failure",` +
	`"artifactId":"sha256:bad","environment":"prod"}`

func stewardStation(k koine.Koine) wire.Station {
	return wire.Station{
		Koine:    k,
		Manifest: []byte(`{"identity":{"name":"deployment-steward"}}`),
		Decode: func(f wire.DeliveryFrame) (koine.Delivery, error) {
			var d deployment.ResolvedDelivery
			if err := d.UnmarshalJSON(f.Event); err != nil {
				return nil, err
			}
			return d, nil
		},
	}
}

// TestWire_TheGuestResolvesADeliveryAndYields is the end-to-end shape driven
// off-target: the host hands over a projected delivery, the station
// resolves, speaks an exchange, waits for the answer, and yields what
// follows from it. The station body is the §10 steward, unmodified.
func TestWire_TheGuestResolvesADeliveryAndYields(t *testing.T) {
	host := &scriptHost{
		answer: wire.AnswerFrame{Status: 200, Value: []byte(`{"artifactId":"sha256:good"}`)},
	}
	guest := wire.New(stewardStation(&station.DeploymentSteward{}), host)

	if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
	}

	if len(host.exchanges) != 1 {
		t.Fatalf("the guest spoke %d exchanges, want one", len(host.exchanges))
	}
	ex := host.exchanges[0]
	if ex.Type != "history.last" {
		t.Errorf("exchange = %#v", ex)
	}
	if ex.Outcome != "success" {
		t.Errorf("the intent asked for outcome %q", ex.Outcome)
	}
	if host.valPolls != 1 {
		t.Errorf("the guest asked for the value %d times; the host's own poll waits", host.valPolls)
	}

	if len(host.yields) != 1 {
		t.Fatalf("the guest yielded %d times, want one: %#v", len(host.yields), host.yields)
	}
	y := host.yields[0]
	if y["type"] != "dev.cdevents.deployment.requested" {
		t.Errorf("the guest yielded %v", y["type"])
	}
	if y["artifact"] != "sha256:good" {
		t.Errorf("the deploy did not carry the last good artifact: %v", y["artifact"])
	}
	if y["target"] != "prod" {
		t.Errorf("the deploy targeted %v", y["target"])
	}
}

// TestWire_ZeroIsSuccessOnTheYieldPath pins the polarity in both directions.
// The host answers ZERO for success; reading that backwards would turn every
// stored emission into a cancellation, silently, without ever raising an
// error — which is exactly the kind of break a version number cannot catch
// and a test in one repository will not see.
func TestWire_ZeroIsSuccessOnTheYieldPath(t *testing.T) {
	t.Run("zero carries on", func(t *testing.T) {
		host := &scriptHost{yieldCode: 0}
		guest := wire.New(stewardStation(&twoSpeaker{}), host)
		if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
			t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
		}
		if len(host.yields) != 2 {
			t.Fatalf("the body spoke twice; the host heard %d", len(host.yields))
		}
	})

	t.Run("non-zero cancels, and nothing after it is spoken", func(t *testing.T) {
		host := &scriptHost{failYieldN: 1}
		guest := wire.New(stewardStation(&twoSpeaker{}), host)
		if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Cancelled {
			t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
		}
		if len(host.yields) != 1 {
			t.Fatalf("the host refused the first yield and still heard %d", len(host.yields))
		}
	})

	// Every non-zero code the host can answer means the same thing: it
	// could not take the utterance.
	for _, code := range []uint32{1, 2, 3} {
		host := &scriptHost{yieldCode: code}
		guest := wire.New(stewardStation(&twoSpeaker{}), host)
		if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Cancelled {
			t.Errorf("host code %d ended %s, want cancelled", code, got)
		}
	}
}

// twoSpeaker speaks twice, so a refusal has something after it to suppress.
type twoSpeaker struct{ koine.ObserverBase }

func (twoSpeaker) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "deployment-steward"}
}
func (twoSpeaker) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (twoSpeaker) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (twoSpeaker) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
	yield(deployment.Deploy{Artifact: dep.ArtifactID, Target: dep.Environment})
}

// TestWire_AwaitIsOneCallBecauseTheHostWaits pins where the waiting lives.
// The host's value_poll does not return until the exchange has gone one way
// or the other, so the guest asks once. A poll loop here would be this SDK
// inventing a mechanism the ruling left to Conduit (E-C, amended).
func TestWire_AwaitIsOneCallBecauseTheHostWaits(t *testing.T) {
	host := &scriptHost{answer: wire.AnswerFrame{Status: 200, Value: []byte(`{"artifactId":"sha256:good"}`)}}
	guest := wire.New(stewardStation(&station.DeploymentSteward{}), host)
	if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
	}
	if host.valPolls != 1 {
		t.Fatalf("the guest asked %d times for one value", host.valPolls)
	}
}

// TestWire_ABreachIsATypedVariantNeverTransport pins that the error a body
// branches on is the future having gone the other way, carrying the host's
// own words.
func TestWire_ABreachIsATypedVariantNeverTransport(t *testing.T) {
	host := &scriptHost{answer: wire.AnswerFrame{
		Status: 404, Error: "no deployment of this subject has ever succeeded", Breached: true,
	}}
	guest := wire.New(stewardStation(&variantSpeaker{}), host)
	if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
	}
	if len(host.yields) != 1 {
		t.Fatalf("the breach branch yielded %d times", len(host.yields))
	}
	if host.yields[0]["artifact"] != "no deployment of this subject has ever succeeded" {
		t.Errorf("the variant did not reach the body: %v", host.yields[0])
	}

	// A breach the host named without setting the flag is the same fact.
	unflagged := &scriptHost{answer: wire.AnswerFrame{Status: 500, Error: "gone"}}
	if got := wire.New(stewardStation(&variantSpeaker{}), unflagged).Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("deliver = %s", got)
	}
	if unflagged.yields[0]["artifact"] != "gone" {
		t.Errorf("an unflagged breach read as filled: %v", unflagged.yields[0])
	}

	// A breach with a status and no words still reads as a breach, and
	// says what it can.
	silent := &scriptHost{answer: wire.AnswerFrame{Status: 504, Breached: true}}
	if got := wire.New(stewardStation(&variantSpeaker{}), silent).Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("deliver = %s", got)
	}
	if !strings.Contains(silent.yields[0]["artifact"].(string), "504") {
		t.Errorf("a wordless breach said %v", silent.yields[0])
	}
}

// variantSpeaker branches on the typed outcome variant and speaks its name,
// so a test can witness what the body was actually handed.
type variantSpeaker struct{ koine.ObserverBase }

func (variantSpeaker) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "deployment-steward"}
}
func (variantSpeaker) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (variantSpeaker) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (variantSpeaker) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	_, err := dep.History().Last(koine.Success).Value()
	if err != nil {
		yield(deployment.DeploymentRecorded{Artifact: deployment.ArtifactRef(err.Error())})
		return
	}
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}

// TestWire_ReceivedIsTheFastBeatAndPendingIsAnHonestNotYet pins the other
// half of the Handle contract, including the honest limit of wire v1: the
// host's ack carries no comprehender, so the only party the guest can name
// is the one that did acknowledge.
func TestWire_ReceivedIsTheFastBeatAndPendingIsAnHonestNotYet(t *testing.T) {
	for _, c := range []struct {
		name  string
		acked bool
		want  koine.ActorRef
	}{
		{"not yet", false, ""},
		{"acknowledged", true, wire.AckBroker},
	} {
		t.Run(c.name, func(t *testing.T) {
			host := &scriptHost{acked: c.acked}
			guest := wire.New(stewardStation(&beatWatcher{}), host)
			if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
				t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
			}
			if host.ackPolls != 1 {
				t.Fatalf("the guest asked the fast beat %d times", host.ackPolls)
			}
			if got := host.yields[0]["artifact"]; got != string(c.want) {
				t.Fatalf("the beat read as %q, want %q", got, c.want)
			}
		})
	}
	if wire.AckBroker == "" {
		t.Error("an acknowledged beat must name someone")
	}
}

// beatWatcher gates on the fast beat and speaks who it names.
type beatWatcher struct{ koine.ObserverBase }

func (beatWatcher) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "deployment-steward"}
}
func (beatWatcher) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (beatWatcher) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (beatWatcher) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	h := dep.History().Last(koine.Success)
	yield(deployment.DeploymentRecorded{Artifact: deployment.ArtifactRef(h.Received().By)})
}

// TestWire_ConstructionIsDelivery pins §4's sentence as a fact: the chain a
// station stands in and the actor whose authority it carries are minted
// below the line and handed up, once, before Resolve.
func TestWire_ConstructionIsDelivery(t *testing.T) {
	steward := &station.DeploymentSteward{}
	if steward.Chain() != "" || steward.Actor() != "" {
		t.Fatal("a station standing nowhere already had a chain")
	}
	host := &scriptHost{answer: wire.AnswerFrame{Status: 200, Value: []byte(`{}`)}}
	guest := wire.New(stewardStation(steward), host)
	if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
	}
	if steward.Chain() != "chain-7" {
		t.Errorf("Chain() = %q, want the chain the frame carried", steward.Chain())
	}
	if steward.Actor() != "sub:mchen;act:conduit" {
		t.Errorf("Actor() = %q, want the actor the host minted", steward.Actor())
	}
	// ProjectContext stays nil: its content is deliberately unruled (§4),
	// and this contract surfaces that rather than inventing a shape.
	if steward.Project() != nil {
		t.Errorf("Project() = %#v; the shape is reserved, not invented", steward.Project())
	}

	// A station that embeds no stratum base is not a station.
	baseless := wire.New(stewardStation(noBase{}), &scriptHost{})
	if got := baseless.Deliver(stewardDelivery(failedDeployment)); got != wire.Refused {
		t.Errorf("a baseless station resolved: %s", got)
	} else if !strings.Contains(baseless.Refusal(), "stratum base") {
		t.Errorf("the refusal did not name the reason: %s", baseless.Refusal())
	}
}

type noBase struct{}

func (noBase) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "deployment-steward"}
}
func (noBase) Awaits() []selector.Selector         { return selector.List(deployment.Resolved()) }
func (noBase) Complete() koine.Contract            { return koine.DefaultAllAwaited }
func (noBase) Resolve(koine.Delivery, koine.Yield) {}

// TestWire_NothingIsStoredOnRefusal pins A9 at the boundary. A frame that
// cannot be read runs no body at all, so nothing is spoken, nothing is
// stored, and the refusal names itself.
func TestWire_NothingIsStoredOnRefusal(t *testing.T) {
	cases := []struct {
		name, frame, wantIn string
	}{
		{
			name:   "a foreign version",
			frame:  `{"version":99,"event":{},"eventType":"x"}`,
			wantIn: "99",
		},
		{
			name:   "an unreadable frame",
			frame:  `{"version":`,
			wantIn: "koine/codec",
		},
		{
			name:   "facts that do not read into this stratum",
			frame:  `{"version":1,"event":{"outcome":7},"eventType":"x"}`,
			wantIn: "did not read into this station's stratum",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			host := &scriptHost{}
			guest := wire.New(stewardStation(&station.DeploymentSteward{}), host)
			if got := guest.Deliver([]byte(c.frame)); got != wire.Refused {
				t.Fatalf("deliver = %s, want refused", got)
			}
			if len(host.yields) != 0 || len(host.exchanges) != 0 {
				t.Fatalf("a refusal still spoke: %d yields, %d exchanges", len(host.yields), len(host.exchanges))
			}
			if !strings.Contains(guest.Refusal(), c.wantIn) {
				t.Fatalf("the refusal did not say %q: %s", c.wantIn, guest.Refusal())
			}
			if !strings.HasPrefix(guest.Refusal(), "koine/wire: ") {
				t.Fatalf("the refusal is anonymous: %s", guest.Refusal())
			}
		})
	}
}

// TestWire_AStoppageBelowTheLineIsNotTheAuthorsTrap is the second review's
// blocker, pinned.
//
// A tool's silence, or a deployment that wired no broker, used to reach the
// record as "trap in resolve" — the author's fault, for a condition that was
// never theirs. It now stops the resolve cooperatively: the reason is spoken
// once through the host's own diagnostic channel with FaultPrefix in front
// of it, every subsequent yield is gated so nothing is stored, and resolve
// answers Unanswered — a number no author can cause.
func TestWire_AStoppageBelowTheLineIsNotTheAuthorsTrap(t *testing.T) {
	cases := map[string]struct {
		host   *scriptHost
		wantIn string
	}{
		"the host would not open the exchange": {
			host: &scriptHost{noHandle: true}, wantIn: "would not open",
		},
		"the host answered with nothing": {
			host: &scriptHost{emptyValue: true}, wantIn: "neither filled nor breached",
		},
		"the host answered with bytes this build cannot read": {
			host: &scriptHost{badValue: true}, wantIn: "cannot read",
		},
		"the fulfiller answered with no value at all": {
			host: &scriptHost{answer: wire.AnswerFrame{Status: 200}}, wantIn: "with no value",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			guest := wire.New(stewardStation(&station.DeploymentSteward{}), c.host)
			got := guest.Deliver(stewardDelivery(failedDeployment))

			if got != wire.Unanswered {
				t.Fatalf("deliver = %s, want unanswered", got)
			}
			if got.AttributedToAuthor() {
				t.Error("a stoppage below the line was attributed to the author")
			}
			if !strings.Contains(guest.Fault(), c.wantIn) {
				t.Fatalf("the fault did not say %q: %s", c.wantIn, guest.Fault())
			}

			// It reaches the record's own diagnostic channel, once,
			// marked so it can be attributed without parsing prose.
			var marked []string
			for _, line := range c.host.logs {
				if strings.HasPrefix(line, wire.FaultPrefix) {
					marked = append(marked, line)
				}
			}
			if len(marked) != 1 {
				t.Fatalf("the host heard %d attributed lines, want one: %v", len(marked), c.host.logs)
			}
			if !strings.Contains(marked[0], c.wantIn) {
				t.Errorf("the logged line said %q", marked[0])
			}

			// And the gate is closed: the steward's variant branch runs
			// and speaks, and nothing it says is stored.
			if len(c.host.yields) != 0 {
				t.Fatalf("a stoppage below the line still stored %#v", c.host.yields)
			}
		})
	}
}

// TestWire_AStoppageIsHandedToTheBodyAsStoppedNotAsAVariant pins the
// distinction the ruling turns on. A Variant is the expected future going
// the other way — a fact about the body's domain, and a valid response. A
// Stopped is a fact about the machinery underneath. A body may branch on
// either; only one of them is about its work.
func TestWire_AStoppageIsHandedToTheBodyAsStoppedNotAsAVariant(t *testing.T) {
	t.Run("a breach is a Variant, and the body's speech is stored", func(t *testing.T) {
		host := &scriptHost{answer: wire.AnswerFrame{
			Status: 404, Error: "no deployment of this subject has ever succeeded", Breached: true,
		}}
		guest := wire.New(stewardStation(&errorNamer{}), host)
		if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
			t.Fatalf("deliver = %s (%s)", got, guest.Fault())
		}
		if len(host.yields) != 1 {
			t.Fatalf("a breach branch stored %d utterances, want one", len(host.yields))
		}
		if got := host.yields[0]["artifact"]; got != "variant:no deployment of this subject has ever succeeded" {
			t.Errorf("the body was handed %v", got)
		}
		if guest.Fault() != "" {
			t.Errorf("a valid response was recorded as a stoppage: %s", guest.Fault())
		}
	})

	t.Run("a stoppage is a Stopped, and nothing is stored", func(t *testing.T) {
		host := &scriptHost{noHandle: true}
		guest := wire.New(stewardStation(&errorNamer{}), host)
		if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Unanswered {
			t.Fatalf("deliver = %s", got)
		}
		if len(host.yields) != 0 {
			t.Fatalf("a stoppage stored %#v", host.yields)
		}
	})
}

// errorNamer speaks the CLASS of whatever error it was handed, so a test can
// witness which of the two the body actually received.
type errorNamer struct{ koine.ObserverBase }

func (errorNamer) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "deployment-steward"}
}
func (errorNamer) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (errorNamer) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (errorNamer) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	_, err := dep.History().Last(koine.Success).Value()
	label := "filled"
	switch e := err.(type) {
	case *wire.Variant:
		label = "variant:" + e.Error()
	case *wire.Stopped:
		label = "stopped:" + e.Why
	}
	yield(deployment.DeploymentRecorded{Artifact: deployment.ArtifactRef(label)})
}

// TestWire_AnAuthorsTrapIsStillTheAuthors keeps the other half of the
// distinction honest. Cooperative stopping is for what the host did; a body
// that cannot be run still traps, and koinehost still records that as the
// author's trap — which is the truth.
func TestWire_AnAuthorsTrapIsStillTheAuthors(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("an ungenerated utterance did not trap")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "no second channel") {
			t.Fatalf("the trap said %v", r)
		}
		if strings.Contains(msg, wire.FaultPrefix) {
			t.Error("an author's trap was dressed up as a stoppage below the line")
		}
	}()
	wire.New(stewardStation(&homemadeSpeaker{}), &scriptHost{}).
		Deliver(stewardDelivery(failedDeployment))
}

// homemade is an utterance an author wrote by hand: it satisfies koine's
// marker, and it can no more cross the wire than a hand-written manifest can
// be believed.
type homemade struct{ koine.IsUtterance }

type homemadeSpeaker struct{ koine.ObserverBase }

func (homemadeSpeaker) Identity() koine.Identity {
	return koine.Identity{Group: "g", Author: "a", Name: "deployment-steward"}
}
func (homemadeSpeaker) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (homemadeSpeaker) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (homemadeSpeaker) Resolve(_ koine.Delivery, yield koine.Yield) {
	yield(homemade{})
}

// TestWire_OffTargetThereIsNoHost pins A7 at the boundary: this SDK ships no
// path to a running engine, and a build that finds itself outside the
// sandbox says so once rather than pretending.
func TestWire_OffTargetThereIsNoHost(t *testing.T) {
	guest := wire.Serve(stewardStation(&station.DeploymentSteward{}))
	for name, call := range map[string]func(){
		"alloc":    func() { guest.AllocExport(16) },
		"manifest": func() { guest.ManifestExport() },
		"resolve":  func() { guest.ResolveExport(0, 0) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("an export answered off-target")
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, "no host below this build") {
					t.Fatalf("said %v", r)
				}
			}()
			call()
		})
	}
	if !strings.Contains(wire.ErrNoHost.Error(), "sandbox") {
		t.Errorf("ErrNoHost = %v", wire.ErrNoHost)
	}
}
