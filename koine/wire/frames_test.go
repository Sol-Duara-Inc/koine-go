// These tests live in wire_test — a FOREIGN package, the way the engine
// sees this contract.
//
// They pin the shapes this SDK writes. They do NOT pin that those shapes are
// the host's — nothing inside one repository can pin that, which is the
// whole lesson of this contract's first attempt. The conformance module
// compares these bytes against the host's own marshaller; what follows is
// the fast inner loop, not the gate.
package wire_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/wire"
)

// TestWire_VersionIsAConstantBothSidesGateOn pins the one thing that makes
// this a contract rather than a hope: the version is a constant, a frame
// that names another one is refused by name, and the refusal quotes both
// sides so the person reading it at load time knows which build to move.
func TestWire_VersionIsAConstantBothSidesGateOn(t *testing.T) {
	if wire.Version != 1 {
		t.Fatalf("this build speaks wire %d; koinehost.WireVersion is 1", wire.Version)
	}
	if err := wire.Accepts(wire.Version); err != nil {
		t.Fatalf("this build refuses its own version: %v", err)
	}

	err := wire.Accepts(99)
	if err == nil {
		t.Fatal("a foreign version was accepted")
	}
	if !errors.Is(err, wire.ErrVersion) {
		t.Errorf("a version refusal must be reachable through ErrVersion: %v", err)
	}
	if !strings.Contains(err.Error(), "99") || !strings.Contains(err.Error(), "1") {
		t.Errorf("the refusal must quote both sides: %v", err)
	}

	// An unversioned frame is refused too, and says so in words rather
	// than by quoting a zero at a person.
	unversioned := wire.Accepts(0)
	if unversioned == nil {
		t.Fatal("an unversioned frame was accepted")
	}
	if !strings.Contains(unversioned.Error(), "no version at all") {
		t.Errorf("an unversioned frame should say so: %v", unversioned)
	}

	// The delivery frame gates itself: the gate is not one place a reader
	// might forget to call.
	foreign, err := wire.DeliveryFrame{Version: 99}.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wire.DecodeDelivery(foreign); !errors.Is(err, wire.ErrVersion) {
		t.Fatalf("a foreign delivery frame was read: %v", err)
	}
	if _, err := wire.DecodeDelivery([]byte(`{}`)); !errors.Is(err, wire.ErrVersion) {
		t.Fatalf("an unversioned delivery frame was read: %v", err)
	}
}

// TestWire_FramesRoundTripWithoutReflection pins the shape of every frame
// byte for byte, in the host's key order and with the host's omitempty
// rules. The literals below are the contract as the engine reads it.
func TestWire_FramesRoundTripWithoutReflection(t *testing.T) {
	t.Run("delivery", func(t *testing.T) {
		want := `{"version":1,"event":{"subject":"payments-api","outcome":"failure"},` +
			`"eventType":"dev.cdevents.deployment.finished","subject":"payments-api",` +
			`"runId":"run-1","chainId":"chain-7","actor":"sub:mchen;act:conduit",` +
			`"context":{"anchor":"deploy","attempt":"2"}}`
		f := wire.DeliveryFrame{
			Version:   wire.Version,
			Event:     []byte(`{"subject":"payments-api","outcome":"failure"}`),
			EventType: "dev.cdevents.deployment.finished",
			Subject:   "payments-api",
			RunID:     "run-1",
			ChainID:   "chain-7",
			Actor:     "sub:mchen;act:conduit",
			Context:   map[string]string{"attempt": "2", "anchor": "deploy"},
		}
		data := render(t, f)
		if data != want {
			t.Fatalf("wrote\n  %s\nwant\n  %s", data, want)
		}
		back, err := wire.DecodeDelivery([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		if back.EventType != f.EventType || back.RunID != f.RunID || back.ChainID != f.ChainID ||
			back.Actor != f.Actor || back.Subject != f.Subject || string(back.Event) != string(f.Event) {
			t.Fatalf("round trip lost keys: %#v", back)
		}
		if back.Anchor() != "deploy" {
			t.Errorf("the anchor did not survive: %q", back.Anchor())
		}
	})

	t.Run("delivery omits what the host omits", func(t *testing.T) {
		const want = `{"version":1,"event":null,"eventType":"","runId":"","chainId":""}`
		if data := render(t, wire.DeliveryFrame{Version: wire.Version}); data != want {
			t.Fatalf("wrote\n  %s\nwant\n  %s", data, want)
		}
		// A context map that exists but is empty is still omitted, the
		// way encoding/json omits it.
		empty := wire.DeliveryFrame{Version: wire.Version, Context: map[string]string{}}
		if data := render(t, empty); strings.Contains(data, "context") {
			t.Errorf("an empty context was written: %s", data)
		}
	})

	t.Run("yield is the domain object, not a wrapper", func(t *testing.T) {
		spoken := deployment.SeedDeploy("sha256:good", "prod")
		want := `{"type":"dev.cdevents.deployment.requested","artifact":"sha256:good","target":"prod"}`
		got := render(t, wire.YieldFrame{Type: spoken.EventType(), Body: spoken})
		if got != want {
			t.Fatalf("wrote\n  %s\nwant\n  %s", got, want)
		}
		// The host stores these very bytes as the payload and reads the
		// type out of them, so a nested "payload" or "body" key would
		// mean the record holds the wire's paperwork instead of the
		// station's speech.
		for _, wrapper := range []string{`"payload"`, `"body"`, `"wire"`} {
			if strings.Contains(got, wrapper) {
				t.Errorf("the yield frame wraps the object: %s", got)
			}
		}
	})

	t.Run("exchange", func(t *testing.T) {
		want := `{"type":"history.last","filter":{"environment":"prod"},"outcome":"success"}`
		f := wire.NewExchangeFrame(koine.Exchange{
			Seat: "history",
			Name: "history.last",
			Args: []koine.Arg{
				{Name: "outcome", Value: "success"},
				{Name: "environment", Value: "prod"},
			},
		})
		if data := render(t, f); data != want {
			t.Fatalf("wrote\n  %s\nwant\n  %s", data, want)
		}
		// An argument named outcome rides the host's own Outcome field
		// and nowhere else — one fact, one place.
		if f.Filter["outcome"] != "" {
			t.Errorf("the outcome argument was written twice: %#v", f)
		}
		if f.Outcome != "success" {
			t.Errorf("the outcome argument did not reach its field: %#v", f)
		}
		back, err := wire.DecodeExchange([]byte(want))
		if err != nil {
			t.Fatal(err)
		}
		if back.Type != "history.last" || back.Outcome != "success" || back.Filter["environment"] != "prod" {
			t.Fatalf("round trip lost keys: %#v", back)
		}

		// An intent with no arguments writes neither key.
		bare := render(t, wire.NewExchangeFrame(koine.Exchange{Seat: "ledger", Name: "ledger.note"}))
		if bare != `{"type":"ledger.note"}` {
			t.Errorf("an argument-less intent wrote %s", bare)
		}
	})

	t.Run("answer", func(t *testing.T) {
		filled := `{"status":200,"value":{"artifactId":"sha256:good"}}`
		if data := render(t, wire.AnswerFrame{Status: 200, Value: []byte(`{"artifactId":"sha256:good"}`)}); data != filled {
			t.Fatalf("wrote\n  %s\nwant\n  %s", data, filled)
		}
		breached := `{"status":404,"error":"nothing good","breached":true}`
		if data := render(t, wire.AnswerFrame{Status: 404, Error: "nothing good", Breached: true}); data != breached {
			t.Fatalf("wrote\n  %s\nwant\n  %s", data, breached)
		}
		back, err := wire.DecodeAnswer([]byte(breached))
		if err != nil {
			t.Fatal(err)
		}
		if !back.Breach() || back.Error != "nothing good" || back.Status != 404 {
			t.Fatalf("round trip lost keys: %#v", back)
		}
		// A named error without the flag is the same fact said once.
		if !(wire.AnswerFrame{Error: "gone"}).Breach() {
			t.Error("an answer that named an error is not filled")
		}
		if (wire.AnswerFrame{Status: 200}).Breach() {
			t.Error("a plain answer is not a breach")
		}
	})
}

// TestWire_FramesCarryPayloadsWithoutParsingThem pins the division of
// labour: a stratum's shape belongs to the stratum, and the wire moves it
// whole. This is also why a delivery may grow keys (conduit-go#184's run
// register) without moving Version.
func TestWire_FramesCarryPayloadsWithoutParsingThem(t *testing.T) {
	odd := `{"a":[1,{"b":"}"},null],"deep":{"deeper":{"deepest":[]}},"n":-1.5e3}`
	f := wire.DeliveryFrame{Version: wire.Version, Event: []byte(odd)}
	back, err := wire.DecodeDelivery([]byte(render(t, f)))
	if err != nil {
		t.Fatal(err)
	}
	if string(back.Event) != odd {
		t.Fatalf("the payload came back as\n  %s\nwant\n  %s", back.Event, odd)
	}
	// A delivery carrying keys this build has never heard of is still a
	// delivery: unknown keys are skipped, not refused.
	grown := `{"version":1,"event":{},"eventType":"x","runId":"r","chainId":"c",` +
		`"runRegister":{"slots":[{"k":"v"}]},"somethingNewer":7}`
	if _, err := wire.DecodeDelivery([]byte(grown)); err != nil {
		t.Fatalf("a delivery that grew keys was refused: %v", err)
	}
}

// TestWire_UnreadableFramesAreRefusedByName keeps the contract on the same
// posture as every other door in this repository: refuse, and say where.
func TestWire_UnreadableFramesAreRefusedByName(t *testing.T) {
	for _, bad := range []string{
		``,
		`{`,
		`{"version":}`,
		`{"version":1} trailing`,
		`[1,2]`,
		`{"version":1,"event":}`,
		`{"version":"one"}`,
	} {
		if _, err := wire.DecodeDelivery([]byte(bad)); err == nil {
			t.Errorf("%q was read", bad)
		}
	}
	if _, err := wire.DecodeAnswer([]byte(`{"status":"ok"}`)); err == nil {
		t.Error("a status that is not a number was read")
	}
	if _, err := wire.DecodeExchange([]byte(`{"filter":["a"]}`)); err == nil {
		t.Error("a filter that is not an object was read")
	}
}

// TestWire_ABIIsOneListBothSidesRead pins that the names on the boundary are
// DATA. The engine's loader gates on these; if either side kept its own copy
// they would drift, and the drift would surface as a load failure nobody
// could explain.
func TestWire_ABIIsOneListBothSidesRead(t *testing.T) {
	if wire.Module != "koine" || wire.Alias != "conduit" {
		t.Errorf("the import modules are %q and %q", wire.Module, wire.Alias)
	}
	wantImports := "ack_poll,deliver,exchange,host_log,value_poll,yield"
	if strings.Join(wire.GuestImports, ",") != wantImports {
		t.Errorf("guest imports = %v, want %v", wire.GuestImports, wantImports)
	}
	wantExports := "alloc,manifest,resolve"
	if strings.Join(wire.GuestExports, ",") != wantExports {
		t.Errorf("guest exports = %v, want %v", wire.GuestExports, wantExports)
	}
	wantForbidden := "emit,emit_result,host_egress"
	if strings.Join(wire.ForbiddenExports, ",") != wantForbidden {
		t.Errorf("forbidden exports = %v, want %v", wire.ForbiddenExports, wantForbidden)
	}

	// The lists are sorted, because a set that is compared by joining is
	// a set that has to have an order.
	for _, list := range [][]string{
		wire.GuestImports, wire.GuestExports, wire.ToolchainExports,
		wire.ForbiddenExports, wire.ForbiddenImports,
	} {
		for i := 1; i < len(list); i++ {
			if list[i-1] >= list[i] {
				t.Errorf("%v is not sorted", list)
				break
			}
		}
	}

	// No name is both required and forbidden — a contradiction no guest
	// could satisfy.
	for _, e := range wire.GuestExports {
		for _, f := range wire.ForbiddenExports {
			if e == f {
				t.Errorf("%q is both required and forbidden", e)
			}
		}
	}

	// Packing is the whole numeric convention on the boundary.
	for _, c := range []struct{ addr, length uint32 }{{0, 0}, {1, 1}, {1 << 31, 4096}, {^uint32(0), ^uint32(0)}} {
		addr, length := wire.Unpack(wire.Pack(c.addr, c.length))
		if addr != c.addr || length != c.length {
			t.Errorf("pack/unpack(%d,%d) = (%d,%d)", c.addr, c.length, addr, length)
		}
	}
}

// TestWire_OutcomesAreNamed pins the small vocabulary a host reads off the
// resolve export, including the one that is deliberately absent.
func TestWire_OutcomesAreNamed(t *testing.T) {
	cases := map[wire.Outcome]string{
		wire.Resolved:   "resolved",
		wire.Cancelled:  "cancelled",
		wire.Refused:    "refused",
		wire.Outcome(9): "unknown",
	}
	for o, want := range cases {
		if o.String() != want {
			t.Errorf("Outcome(%d) = %q, want %q", uint32(o), o.String(), want)
		}
	}
	// Zero is success on both sides of the boundary: the host reads
	// anything else as work.finished{outcome: failure}.
	if wire.Resolved != 0 {
		t.Error("resolved must be zero — the host reads non-zero as failure")
	}
	if wire.Cancelled == 0 || wire.Refused == 0 {
		t.Error("cancelled and refused must not read as success")
	}
}

func render(t *testing.T, f interface{ MarshalJSON() ([]byte, error) }) string {
	t.Helper()
	data, err := f.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
