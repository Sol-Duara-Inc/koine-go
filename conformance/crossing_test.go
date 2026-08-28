// Package conformance is the test that crosses the repository boundary.
//
// Both halves of K2 passed their own suites completely, and that is exactly
// why they shipped against two different readings of one wire: every
// mismatch between them was invisible to any test that stayed in one repo.
// This module exists so that can never happen again. It builds the committed
// fixture guests with the toolchain the engine uses, hands the bytes to the
// REAL koinehost.Host.Load, and runs a real delivery through a real broker.
//
// It is a separate Go module because koine-go's own module has no
// dependencies and no go.sum. Go excludes a directory with its own go.mod
// from the parent's package patterns, so the zero-dependency law is
// untouched and this module is invisible to `go build ./...` at the root.
//
//	cd conformance && go test ./...
//
// It needs a conduit-go checkout beside the koine-go one, and TinyGo on
// PATH. Both are what CI provides.
package conformance_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sol-duara-inc/koine-go/koine/wire"
	"github.com/solduara/conduit-go/pkg/koinehost"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller info")
	}
	return filepath.Dir(filepath.Dir(self)) // conformance -> repo root
}

// buildGuest compiles a fixture guest exactly as the engine would.
func buildGuest(t *testing.T, pkg string) []byte {
	t.Helper()
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("tinygo is not installed: the crossing test cannot build a guest, and it proves nothing without one")
	}
	out := filepath.Join(t.TempDir(), filepath.Base(pkg)+".wasm")
	cmd := exec.Command("tinygo", "build", "-o", out, "-target", "wasm-unknown", "./"+pkg)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s does not build under TinyGo:\n%s", pkg, combined)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func newHost(t *testing.T) (*koinehost.Host, context.Context) {
	t.Helper()
	ctx := context.Background()
	h, err := koinehost.NewHost(ctx, koinehost.Options{Timeout: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close(context.Background()) })
	return h, ctx
}

// TestCrossing_TheStewardLoadsIntoTheRealHost is the done-condition the
// review named: the committed fixture guest, built with the pinned
// toolchain, handed to the merged loader. Everything Load enforces — the
// export set and its signatures, the import allow-list, the manifest's shape
// — is enforced here by the loader itself rather than by this repository's
// opinion of it.
func TestCrossing_TheStewardLoadsIntoTheRealHost(t *testing.T) {
	h, ctx := newHost(t)
	station, err := h.Load(ctx, "deployment-steward", buildGuest(t, "fixtures/guest/steward"))
	if err != nil {
		t.Fatalf("the loader refused the steward: %v", err)
	}
	defer station.Close(context.Background())

	m := station.Manifest()
	if m == nil {
		t.Fatal("the loader read no manifest — the manifest export is the engine's existing door (A2)")
	}
	if m.Identity.Name != "deployment-steward" {
		t.Errorf("identity.name = %q", m.Identity.Name)
	}
	if m.Identity.Group != "payment-engineering" || m.Identity.Author != "mchen" {
		t.Errorf("identity = %#v", m.Identity)
	}
	if m.Claim != "dev.cdevents.deployment" {
		t.Errorf("claim = %q — the loader holds a claim to a reverse-domain shape", m.Claim)
	}
	if m.Complete != "all-awaited" {
		t.Errorf("complete = %q", m.Complete)
	}
	if strings.Join(m.Awaits, ",") != "dev.cdevents.deployment" {
		t.Errorf("awaits = %v", m.Awaits)
	}
	if strings.Join(m.Emits, ",") != "dev.cdevents.deployment.recorded,dev.cdevents.deployment.requested" {
		t.Errorf("emits = %v", m.Emits)
	}
	if strings.Join(m.Exchanges, ",") != "history.last" {
		t.Errorf("exchanges = %v", m.Exchanges)
	}
}

// TestCrossing_ADeliveryRoundTrips is the other half of the done-condition:
// one delivery in, the station's own speech out, formed and stored by the
// host. The station body is the §10 steward, unmodified — the same body the
// manifest extractor reads and the pure-Resolve harness drives.
func TestCrossing_ADeliveryRoundTrips(t *testing.T) {
	h, ctx := newHost(t)
	station, err := h.Load(ctx, "deployment-steward", buildGuest(t, "fixtures/guest/steward"))
	if err != nil {
		t.Fatal(err)
	}
	defer station.Close(context.Background())

	broker := koinehost.NewMemoryBroker()
	broker.RegisterHandler("history.last", func(req koinehost.ExchangeRequest) koinehost.ExchangeResponse {
		if req.Outcome != "success" {
			t.Errorf("the intent asked for outcome %q; the resolved idiom asks the record for the last success", req.Outcome)
		}
		return koinehost.ExchangeResponse{
			Status: 200,
			Value:  json.RawMessage(`{"artifactId":"sha256:good"}`),
		}
	})
	emitter := koinehost.NewRecordingEmitter()

	res, err := station.Run(ctx, koinehost.Invocation{
		Declared: true,
		Emitter:  emitter,
		Broker:   broker,
		Delivery: koinehost.Delivery{
			Version:   koinehost.WireVersion,
			EventType: "dev.cdevents.deployment.finished",
			Subject:   "payments-api",
			RunID:     "run-crossing-1",
			ChainID:   "chain-crossing-1",
			Actor:     "sub:mchen;act:conduit",
			Context:   map[string]string{wire.ContextAnchor: "deploy"},
			Event: json.RawMessage(`{"subject":"payments-api","outcome":"failure",` +
				`"artifactId":"sha256:bad","environment":"prod"}`),
		},
	})
	if err != nil {
		t.Fatalf("the station did not run: %v (logs %v)", err, res.Logs)
	}
	if res.Status != 0 {
		t.Fatalf("resolve answered %d; zero is success", res.Status)
	}

	if len(res.Yields) != 1 {
		t.Fatalf("the failure branch spoke %d utterances, want one: %#v", len(res.Yields), res.Yields)
	}
	y := res.Yields[0]
	if y.Type != "dev.cdevents.deployment.requested" {
		t.Errorf("the host read the event type as %q", y.Type)
	}

	// What the host STORES is the payload, and the payload must be the
	// domain object — not an envelope around it. A wrapper here would mean
	// the record holds the wire's paperwork instead of the station's
	// speech.
	var spoke map[string]any
	if err := json.Unmarshal(y.Payload, &spoke); err != nil {
		t.Fatalf("the stored payload is not readable: %v", err)
	}
	if spoke["artifact"] != "sha256:good" {
		t.Errorf("the deploy did not carry the last good artifact: %v", spoke["artifact"])
	}
	if spoke["target"] != "prod" {
		t.Errorf("the deploy targeted %v", spoke["target"])
	}
	if _, wrapped := spoke["payload"]; wrapped {
		t.Errorf("the stored payload is an envelope, not the domain object: %s", y.Payload)
	}
	if _, wrapped := spoke["body"]; wrapped {
		t.Errorf("the stored payload is an envelope, not the domain object: %s", y.Payload)
	}

	// The envelope the host formed carries the chain and the actor the
	// delivery named — construction is delivery, all the way down.
	if y.ChainID != "chain-crossing-1" || y.RunID != "run-crossing-1" {
		t.Errorf("envelope = %#v", y)
	}
	if y.Actor != "sub:mchen;act:conduit" {
		t.Errorf("envelope actor = %q", y.Actor)
	}

	// Auto-emission around the guest's lifecycle is the host's, and the
	// run finished successfully — which is what a correctly-read yield
	// return looks like from up here.
	var finished *koinehost.WorkEvent
	for i := range emitter.WorkEvents {
		if emitter.WorkEvents[i].Type == koinehost.WorkFinished {
			finished = &emitter.WorkEvents[i]
		}
	}
	if finished == nil {
		t.Fatalf("no work.finished was emitted: %#v", emitter.WorkEvents)
	}
	if finished.Outcome != koinehost.OutcomeSuccess {
		t.Fatalf("work.finished{outcome: %q, error: %q}", finished.Outcome, finished.Error)
	}
}

// TestCrossing_TheSuccessBranchSpeaksWithoutAnExchange drives the other
// branch through the real host, so the crossing covers the body's whole
// shape rather than one path through it.
func TestCrossing_TheSuccessBranchSpeaksWithoutAnExchange(t *testing.T) {
	h, ctx := newHost(t)
	station, err := h.Load(ctx, "deployment-steward", buildGuest(t, "fixtures/guest/steward"))
	if err != nil {
		t.Fatal(err)
	}
	defer station.Close(context.Background())

	broker := koinehost.NewMemoryBroker()
	spoken := 0
	var mu sync.Mutex
	broker.RegisterHandler("history.last", func(koinehost.ExchangeRequest) koinehost.ExchangeResponse {
		mu.Lock()
		spoken++
		mu.Unlock()
		return koinehost.ExchangeResponse{Status: 200, Value: json.RawMessage(`{}`)}
	})

	res, err := station.Run(ctx, koinehost.Invocation{
		Declared: true,
		Emitter:  koinehost.NewRecordingEmitter(),
		Broker:   broker,
		Delivery: koinehost.Delivery{
			Version:   koinehost.WireVersion,
			EventType: "dev.cdevents.deployment.finished",
			RunID:     "run-crossing-2",
			ChainID:   "chain-crossing-2",
			Event:     json.RawMessage(`{"outcome":"success","artifactId":"sha256:fine"}`),
		},
	})
	if err != nil || res.Status != 0 {
		t.Fatalf("run = %d, %v (logs %v)", res.Status, err, res.Logs)
	}
	if spoken != 0 {
		t.Errorf("the success branch spoke %d exchanges; it needs none", spoken)
	}
	if len(res.Yields) != 1 || res.Yields[0].Type != "dev.cdevents.deployment.recorded" {
		t.Fatalf("the success branch spoke %#v", res.Yields)
	}
}

// TestCrossing_ABreachedExchangeIsBranchedOn pins the typed outcome variant
// across the real boundary: the future went the other way, the body branched,
// and nothing about it was transport.
func TestCrossing_ABreachedExchangeIsBranchedOn(t *testing.T) {
	h, ctx := newHost(t)
	station, err := h.Load(ctx, "deployment-steward", buildGuest(t, "fixtures/guest/steward"))
	if err != nil {
		t.Fatal(err)
	}
	defer station.Close(context.Background())

	broker := koinehost.NewMemoryBroker()
	broker.RegisterHandler("history.last", func(koinehost.ExchangeRequest) koinehost.ExchangeResponse {
		return koinehost.ExchangeResponse{
			Status:   404,
			Error:    "no deployment of this subject has ever succeeded",
			Breached: true,
		}
	})

	res, err := station.Run(ctx, koinehost.Invocation{
		Declared: true,
		Emitter:  koinehost.NewRecordingEmitter(),
		Broker:   broker,
		Delivery: koinehost.Delivery{
			Version:   koinehost.WireVersion,
			EventType: "dev.cdevents.deployment.finished",
			RunID:     "run-crossing-3",
			ChainID:   "chain-crossing-3",
			Event: json.RawMessage(`{"outcome":"failure","artifactId":"sha256:bad",` +
				`"environment":"prod"}`),
		},
	})
	if err != nil || res.Status != 0 {
		t.Fatalf("run = %d, %v (logs %v)", res.Status, err, res.Logs)
	}
	if len(res.Yields) != 1 || res.Yields[0].Type != "dev.cdevents.deployment.recorded" {
		t.Fatalf("the breach branch spoke %#v", res.Yields)
	}
}

// TestCrossing_TheSecondPathFixtureIsRefusedByName is the negative fixture
// meeting the real refusal. A guest that declares a second emit path is
// refused at Load, before it is ever instantiated, and the refusal names the
// export.
func TestCrossing_TheSecondPathFixtureIsRefusedByName(t *testing.T) {
	h, ctx := newHost(t)
	station, err := h.Load(ctx, "secondpath", buildGuest(t, "fixtures/guest/secondpath"))
	if err == nil {
		_ = station.Close(context.Background())
		t.Fatal("a guest declaring a second emit path was loaded")
	}
	if !strings.Contains(err.Error(), "emit") {
		t.Errorf("the refusal did not name the export: %v", err)
	}
	if !strings.Contains(err.Error(), "yield") {
		t.Errorf("the refusal did not name the only permitted path: %v", err)
	}
}

// TestCrossing_AnUndeclaredStationReceivesNothing pins "declared, never
// ambient" (ruled 2026-08-26, DESIGN §7) at the boundary: a loaded station
// that no workflow declares receives no delivery, speaks nothing, and routes
// nothing. Presence in a deployment grants nothing.
func TestCrossing_AnUndeclaredStationReceivesNothing(t *testing.T) {
	h, ctx := newHost(t)
	station, err := h.Load(ctx, "deployment-steward", buildGuest(t, "fixtures/guest/steward"))
	if err != nil {
		t.Fatal(err)
	}
	defer station.Close(context.Background())

	emitter := koinehost.NewRecordingEmitter()
	res, err := station.Run(ctx, koinehost.Invocation{
		Declared: false, // nothing declares it
		Emitter:  emitter,
		Broker:   koinehost.NewMemoryBroker(),
		Delivery: koinehost.Delivery{
			Version:   koinehost.WireVersion,
			EventType: "dev.cdevents.deployment.finished",
			RunID:     "run-crossing-4",
			ChainID:   "chain-crossing-4",
			Event:     json.RawMessage(`{"outcome":"success","artifactId":"sha256:fine"}`),
		},
	})
	if err != nil {
		t.Fatalf("an undeclared station errored rather than simply not running: %v", err)
	}
	if len(res.Yields) != 0 || len(emitter.Yields) != 0 {
		t.Errorf("an undeclared station spoke: %#v", res.Yields)
	}
	if len(emitter.WorkEvents) != 0 {
		t.Errorf("an undeclared station produced lifecycle events: %#v", emitter.WorkEvents)
	}
}

// TestCrossing_TheWireVersionsAgree is the cheapest gate of all, and the one
// whose absence let two halves ship against two readings of one wire.
func TestCrossing_TheWireVersionsAgree(t *testing.T) {
	if wire.Version != koinehost.WireVersion {
		t.Fatalf("koine/wire speaks %d; koinehost speaks %d — the two halves are not the same wire",
			wire.Version, koinehost.WireVersion)
	}
}

// TestCrossing_TheFrameShapesAgree compares this SDK's hand-rolled,
// reflection-free encoders against the host's own encoding/json output, byte
// for byte. Every frame the guest writes and every frame it reads is one the
// host wrote or will read; if the two marshallers disagree about a key, an
// order, or an omitempty, this is where it shows.
func TestCrossing_TheFrameShapesAgree(t *testing.T) {
	t.Run("delivery", func(t *testing.T) {
		hostSide := koinehost.Delivery{
			Version:   koinehost.WireVersion,
			Event:     json.RawMessage(`{"outcome":"failure"}`),
			EventType: "dev.cdevents.deployment.finished",
			Subject:   "payments-api",
			RunID:     "run-1",
			ChainID:   "chain-1",
			Actor:     "sub:mchen",
			Context:   map[string]string{"anchor": "deploy", "attempt": "2"},
		}
		want, err := json.Marshal(hostSide)
		if err != nil {
			t.Fatal(err)
		}
		guestSide := wire.DeliveryFrame{
			Version:   wire.Version,
			Event:     []byte(`{"outcome":"failure"}`),
			EventType: "dev.cdevents.deployment.finished",
			Subject:   "payments-api",
			RunID:     "run-1",
			ChainID:   "chain-1",
			Actor:     "sub:mchen",
			Context:   map[string]string{"anchor": "deploy", "attempt": "2"},
		}
		got, err := guestSide.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("the two halves render a delivery differently:\n guest %s\n  host %s", got, want)
		}

		// And the guest reads what the host writes.
		back, err := wire.DecodeDelivery(want)
		if err != nil {
			t.Fatal(err)
		}
		if back.EventType != hostSide.EventType || back.ChainID != hostSide.ChainID ||
			back.Actor != hostSide.Actor || back.Anchor() != "deploy" ||
			string(back.Event) != string(hostSide.Event) {
			t.Fatalf("the guest read the host's delivery as %#v", back)
		}
	})

	t.Run("delivery with everything omitted", func(t *testing.T) {
		want, err := json.Marshal(koinehost.Delivery{Version: koinehost.WireVersion})
		if err != nil {
			t.Fatal(err)
		}
		got, err := wire.DeliveryFrame{Version: wire.Version}.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("omitempty disagrees:\n guest %s\n  host %s", got, want)
		}
	})

	t.Run("exchange request", func(t *testing.T) {
		want, err := json.Marshal(koinehost.ExchangeRequest{
			Type:    "history.last",
			Filter:  map[string]string{"environment": "prod"},
			Outcome: "success",
		})
		if err != nil {
			t.Fatal(err)
		}
		got, err := wire.ExchangeFrame{
			Type:    "history.last",
			Filter:  map[string]string{"environment": "prod"},
			Outcome: "success",
		}.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("the two halves render an exchange differently:\n guest %s\n  host %s", got, want)
		}
	})

	t.Run("exchange response", func(t *testing.T) {
		for _, hostSide := range []koinehost.ExchangeResponse{
			{Status: 200, Value: json.RawMessage(`{"artifactId":"sha256:good"}`)},
			{Status: 404, Error: "nothing good", Breached: true},
			{Status: 0},
		} {
			want, err := json.Marshal(hostSide)
			if err != nil {
				t.Fatal(err)
			}
			got, err := wire.AnswerFrame{
				Status:   hostSide.Status,
				Value:    []byte(hostSide.Value),
				Error:    hostSide.Error,
				Breached: hostSide.Breached,
			}.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Fatalf("the two halves render a response differently:\n guest %s\n  host %s", got, want)
			}
			back, err := wire.DecodeAnswer(want)
			if err != nil {
				t.Fatal(err)
			}
			if back.Status != hostSide.Status || back.Error != hostSide.Error || back.Breached != hostSide.Breached {
				t.Fatalf("the guest read the host's response as %#v", back)
			}
		}
	})
}

// TestCrossing_TheManifestIsTheHostsShape reads the committed manifest
// golden with the HOST's own unmarshaller and validator, so the extended
// Koine section cannot quietly break the half the loader reads.
func TestCrossing_TheManifestIsTheHostsShape(t *testing.T) {
	for _, guest := range []string{"steward", "secondpath"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "fixtures", "guest", guest, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m koinehost.Manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: the host cannot read the manifest: %v", guest, err)
		}
		if m.Identity.Name == "" && m.Claim == "" {
			t.Errorf("%s: the host would refuse this manifest as declaring neither name nor claim", guest)
		}
		if len(raw) > 64*1024 {
			t.Errorf("%s: the manifest is %d bytes; the loader bounds it at 65536", guest, len(raw))
		}
	}
}
