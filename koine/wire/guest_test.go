package wire_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/station"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
	"github.com/sol-duara-inc/koine-go/koine/wire"
)

// scriptHost is a host written from a script: it records what the guest said
// and answers what the test told it to. It is not a mock of the semantics —
// there are no semantics on this side of the boundary, only frames — which
// is exactly why the guest runtime can be driven here at all.
type scriptHost struct {
	// what the guest said
	yields    []wire.YieldFrame
	exchanges []wire.ExchangeFrame
	ackPolls  int
	valPolls  int

	// what this host answers
	cancelAt  int // 1-based yield to refuse; 0 refuses nothing
	token     uint64
	openErr   string
	pendings  int // value polls answered "pending" before the real answer
	value     wire.ValueFrame
	ackAfter  int // ack polls answered "pending" before "received"
	ackBy     string
	badFrames bool
	noToken   bool // open the exchange with no token at all
}

func (h *scriptHost) Yield(frame []byte) bool {
	f, err := wire.DecodeYield(frame)
	if err != nil {
		panic("the guest sent an unreadable yield frame: " + err.Error())
	}
	h.yields = append(h.yields, f)
	return h.cancelAt == 0 || len(h.yields) != h.cancelAt
}

func (h *scriptHost) Exchange(frame []byte) []byte {
	f, err := wire.DecodeExchange(frame)
	if err != nil {
		panic("the guest sent an unreadable exchange frame: " + err.Error())
	}
	h.exchanges = append(h.exchanges, f)
	if h.badFrames {
		return []byte(`{"wire":"koine.wire/99"}`)
	}
	token := h.token
	if token == 0 && h.openErr == "" && !h.noToken {
		token = uint64(len(h.exchanges))
	}
	return mustRender(wire.OpenedFrame{Wire: wire.Version, Token: token, Err: h.openErr})
}

func (h *scriptHost) AckPoll(uint64) []byte {
	h.ackPolls++
	state, by := wire.StateReceived, h.ackBy
	if h.ackPolls <= h.ackAfter {
		state, by = wire.StatePending, ""
	}
	return mustRender(wire.AckFrame{Wire: wire.Version, State: state, By: by})
}

func (h *scriptHost) ValuePoll(uint64) []byte {
	h.valPolls++
	if h.valPolls <= h.pendings {
		return mustRender(wire.ValueFrame{Wire: wire.Version, State: wire.StatePending})
	}
	v := h.value
	v.Wire = wire.Version
	if v.State == "" {
		// A host that was given no script still answers something a
		// conforming guest can read, so a test that is not about waiting
		// never trips over the wait.
		v.State = wire.StateFilled
	}
	return mustRender(v)
}

// mustRender builds an answer frame for the script host. A frame this
// package just built failing to render itself is a bug in this package, not
// a condition a test has to carry.
func mustRender(f interface{ MarshalJSON() ([]byte, error) }) []byte {
	data, err := f.MarshalJSON()
	if err != nil {
		panic("koine/wire test: a frame could not render itself: " + err.Error())
	}
	return data
}

func stewardDelivery(facts string) []byte {
	frame, err := wire.DeliveryFrame{
		Wire:    wire.Version,
		Station: "deployment-steward",
		Chain:   "chain/7",
		Actor:   "sub:mchen;act:conduit",
		Anchor:  "deploy",
		Type:    "dev.cdevents.deployment.finished",
		Facts:   []byte(facts),
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
		Manifest: []byte(`{"schemaVersion":"koine.manifest/1"}`),
		Decode: func(f wire.DeliveryFrame) (koine.Delivery, error) {
			var d deployment.ResolvedDelivery
			if err := d.UnmarshalJSON(f.Facts); err != nil {
				return nil, err
			}
			return d, nil
		},
	}
}

// TestWire_TheGuestResolvesADeliveryAndYields is the end-to-end shape K2
// exists to prove, driven off-target: the host hands over a projected
// delivery, the station resolves, speaks an exchange, waits for the answer,
// and yields what follows from it. The station body is the §10 steward,
// unmodified — the same body the extractor reads and the harness drives.
func TestWire_TheGuestResolvesADeliveryAndYields(t *testing.T) {
	host := &scriptHost{
		pendings: 2,
		value: wire.ValueFrame{
			State: wire.StateFilled,
			By:    "sub:history-fulfiller",
			Body:  []byte(`{"artifactId":"sha256:good"}`),
		},
	}
	steward := &station.DeploymentSteward{}
	guest := wire.New(stewardStation(steward), host)

	if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
	}

	if len(host.exchanges) != 1 {
		t.Fatalf("the guest spoke %d exchanges, want one", len(host.exchanges))
	}
	ex := host.exchanges[0]
	if ex.Seat != "history" || ex.Name != "history.last" {
		t.Errorf("exchange = %#v", ex)
	}
	if len(ex.Args) != 1 || ex.Args[0].Name != "outcome" || ex.Args[0].Value != "success" {
		t.Errorf("the intent carried %#v", ex.Args)
	}

	if len(host.yields) != 1 {
		t.Fatalf("the guest yielded %d times, want one: %#v", len(host.yields), host.yields)
	}
	y := host.yields[0]
	if y.Wire != wire.Version {
		t.Errorf("the yield frame declared %q", y.Wire)
	}
	if y.Type != "dev.cdevents.deployment.requested" {
		t.Errorf("the guest yielded %q", y.Type)
	}
	const wantBody = `{"artifact":"sha256:good","target":"prod"}`
	if string(y.Body) != wantBody {
		t.Errorf("the guest yielded\n  %s\nwant\n  %s", y.Body, wantBody)
	}
}

// TestWire_ValueWaitsUntilFilledOrBreached pins E-C as amended: the guest
// keeps asking until there is news. The loop is deliberately the dumbest
// possible guest, which is what leaves the mechanism to the host — a host
// that suspends and resumes across the boundary never answers "pending" at
// all, and this test is what proves the guest tolerates one that does.
func TestWire_ValueWaitsUntilFilledOrBreached(t *testing.T) {
	t.Run("filled after waiting", func(t *testing.T) {
		host := &scriptHost{
			pendings: 5,
			value: wire.ValueFrame{
				State: wire.StateFilled,
				Body:  []byte(`{"artifactId":"sha256:good"}`),
			},
		}
		guest := wire.New(stewardStation(&station.DeploymentSteward{}), host)
		if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
			t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
		}
		if host.valPolls != 6 {
			t.Errorf("the guest asked %d times, want 6 — five pending and the answer", host.valPolls)
		}
		if string(host.yields[0].Body) != `{"artifact":"sha256:good","target":"prod"}` {
			t.Errorf("the guest read past a value that had not arrived: %s", host.yields[0].Body)
		}
	})

	t.Run("breached is a typed variant, never transport", func(t *testing.T) {
		host := &scriptHost{
			pendings: 1,
			value:    wire.ValueFrame{State: wire.StateBreached, Err: "no-deployment-of-this-subject-ever-succeeded"},
		}
		guest := wire.New(stewardStation(&variantSpeaker{}), host)
		if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
			t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
		}
		if len(host.yields) != 1 {
			t.Fatalf("the breach branch yielded %d times", len(host.yields))
		}
		// The station branched on the variant and spoke its name: the
		// error crossed the boundary as domain truth, not as a transport
		// complaint the author would have to defend against.
		if !strings.Contains(string(host.yields[0].Body), "no-deployment-of-this-subject-ever-succeeded") {
			t.Errorf("the variant did not reach the body: %s", host.yields[0].Body)
		}
	})
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
// half of the Handle contract. Nothing is an error here: only the party who
// declared comprehension receives, and they may simply not have received
// yet.
func TestWire_ReceivedIsTheFastBeatAndPendingIsAnHonestNotYet(t *testing.T) {
	host := &scriptHost{ackAfter: 1, ackBy: "sub:history-fulfiller", value: wire.ValueFrame{State: wire.StateFilled}}
	guest := wire.New(stewardStation(&beatWatcher{}), host)
	if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
	}
	if host.ackPolls != 2 {
		t.Fatalf("the guest asked the fast beat %d times, want 2", host.ackPolls)
	}
	if len(host.yields) != 1 {
		t.Fatalf("yielded %d times", len(host.yields))
	}
	const want = `{"artifact":"pending-then-sub:history-fulfiller"}`
	if string(host.yields[0].Body) != want {
		t.Fatalf("the beats read as %s, want %s", host.yields[0].Body, want)
	}
}

// beatWatcher gates on the fast beat twice and speaks what it saw.
type beatWatcher struct{ koine.ObserverBase }

func (beatWatcher) Identity() koine.Identity {
	return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "deployment-steward"}
}
func (beatWatcher) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (beatWatcher) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (beatWatcher) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	h := dep.History().Last(koine.Success)
	first := h.Received().By
	if first == "" {
		first = "pending"
	}
	yield(deployment.DeploymentRecorded{Artifact: deployment.ArtifactRef(string(first) + "-then-" + string(h.Received().By))})
}

// TestWire_ARefusedYieldCancelsResolution pins the Yield contract across the
// boundary: false is the host cancelling, and nothing after the refusal is
// spoken.
func TestWire_ARefusedYieldCancelsResolution(t *testing.T) {
	host := &scriptHost{cancelAt: 1}
	guest := wire.New(stewardStation(&twoSpeaker{}), host)
	if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Cancelled {
		t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
	}
	if len(host.yields) != 1 {
		t.Fatalf("the host refused the first yield and still heard %d: %#v", len(host.yields), host.yields)
	}

	// With nothing refused, the same body speaks twice — without the
	// control, the count above would prove nothing.
	open := &scriptHost{}
	if got := wire.New(stewardStation(&twoSpeaker{}), open).Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("the control ended %s", got)
	}
	if len(open.yields) != 2 {
		t.Fatalf("the control spoke %d times, want 2", len(open.yields))
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

// TestWire_ConstructionIsDelivery pins §4's sentence as a fact: the chain a
// station stands in and the actor whose authority it carries are minted
// below the line and handed up, once, before Resolve. A station observes
// them; it never invents one, and there is no constructor for it to reach.
func TestWire_ConstructionIsDelivery(t *testing.T) {
	steward := &station.DeploymentSteward{}
	if steward.Chain() != "" || steward.Actor() != "" {
		t.Fatal("a station standing nowhere already had a chain")
	}
	host := &scriptHost{value: wire.ValueFrame{State: wire.StateFilled, Body: []byte(`{}`)}}
	guest := wire.New(stewardStation(steward), host)
	if got := guest.Deliver(stewardDelivery(failedDeployment)); got != wire.Resolved {
		t.Fatalf("deliver = %s (%s)", got, guest.Refusal())
	}
	if steward.Chain() != "chain/7" {
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

	// A station that embeds no stratum base is not a station, and the
	// guest says so instead of resolving it.
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
			frame:  `{"wire":"koine.wire/99","station":"deployment-steward","facts":{}}`,
			wantIn: "koine.wire/99",
		},
		{
			name:   "an unreadable frame",
			frame:  `{"wire":`,
			wantIn: "koine/codec",
		},
		{
			name:   "a frame addressed to another station",
			frame:  `{"wire":"koine.wire/1","station":"some-other-station","facts":{}}`,
			wantIn: "some-other-station",
		},
		{
			name:   "facts that do not read into this stratum",
			frame:  `{"wire":"koine.wire/1","station":"deployment-steward","facts":{"outcome":7}}`,
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

// TestWire_AHostThatCannotOpenAnExchangeIsLoud pins that a seat which will
// not open is not a domain outcome to branch on. It is a deployment that
// registration should already have refused by name (§7.3), and the guest
// traps rather than handing the body a zero value it would treat as an
// answer. The host attributes the trap (§8).
func TestWire_AHostThatCannotOpenAnExchangeIsLoud(t *testing.T) {
	cases := map[string]*scriptHost{
		"the seat refused to open":                        {openErr: "nobody-connected: history"},
		"the host minted no token":                        {noToken: true},
		"the host spoke a version this build cannot read": {badFrames: true},
	}
	for name, host := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("the guest carried on without an exchange to gate on")
				}
				msg, ok := r.(string)
				if !ok || !strings.Contains(msg, "koine/wire") || !strings.Contains(msg, "history.last") {
					t.Fatalf("the trap said %v", r)
				}
			}()
			guest := wire.New(stewardStation(&station.DeploymentSteward{}), host)
			guest.Deliver(stewardDelivery(failedDeployment))
		})
	}
}

// TestWire_TheOnlyEmitPathIsYield pins the claim §8 makes, at the only place
// this side of the boundary can pin it: a station that speaks something no
// stratum generated does not get a second channel to speak it through — it
// traps, and the host attributes the trap.
func TestWire_TheOnlyEmitPathIsYield(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("an ungenerated utterance crossed the boundary")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "no second channel") {
			t.Fatalf("the trap said %v", r)
		}
	}()
	guest := wire.New(stewardStation(&homemadeSpeaker{}), &scriptHost{})
	guest.Deliver(stewardDelivery(failedDeployment))
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
		"manifest": func() { guest.ManifestExport() },
		"inbox":    func() { guest.InboxExport() },
		"deliver":  func() { guest.DeliverExport(0) },
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

// TestWire_InboxCapacityIsAnAnswerNotAnAssumption pins that the size is the
// export's to state. A guest built against a later SDK may carry a different
// one, and the host reads it rather than trusting a constant.
func TestWire_InboxCapacityIsAnAnswerNotAnAssumption(t *testing.T) {
	if wire.InboxCapacity < 4096 {
		t.Errorf("the inbox is %d bytes, which will not hold a manifest", wire.InboxCapacity)
	}
	if strconv.FormatUint(uint64(wire.InboxCapacity), 10) == "" {
		t.Fatal("unreachable")
	}
}
