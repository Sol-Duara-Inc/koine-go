// These tests live in wire_test — a FOREIGN package, the way the engine
// sees this contract. Everything asserted here is asserted from outside the
// wall, because outside the wall is where the other half of the treaty
// stands.
package wire_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/wire"
)

// TestWire_VersionIsAConstantBothSidesGateOn pins the one thing that makes
// this a contract rather than a hope: the version is a constant, a frame
// that names another one is refused by name, and the refusal quotes both
// sides so the person reading it at load time knows which build to move.
func TestWire_VersionIsAConstantBothSidesGateOn(t *testing.T) {
	if wire.Version == "" {
		t.Fatal("the contract has no version")
	}
	if err := wire.Accepts(wire.Version); err != nil {
		t.Fatalf("this build refuses its own version: %v", err)
	}

	err := wire.Accepts("koine.wire/99")
	if err == nil {
		t.Fatal("a foreign version was accepted")
	}
	if !errors.Is(err, wire.ErrVersion) {
		t.Errorf("a version refusal must be reachable through ErrVersion: %v", err)
	}
	if !strings.Contains(err.Error(), "koine.wire/99") || !strings.Contains(err.Error(), wire.Version) {
		t.Errorf("the refusal must quote both sides: %v", err)
	}

	// An unversioned frame is refused too, and says so in words rather
	// than by quoting an empty string at a person.
	unversioned := wire.Accepts("")
	if unversioned == nil {
		t.Fatal("an unversioned frame was accepted")
	}
	if !strings.Contains(unversioned.Error(), "nothing") {
		t.Errorf("an unversioned frame should say so: %v", unversioned)
	}
}

// TestWire_EveryFrameGatesItsOwnVersion pins that the gate is not one place
// a reader might forget to call: every frame in the contract checks itself.
func TestWire_EveryFrameGatesItsOwnVersion(t *testing.T) {
	foreign := map[string]func() ([]byte, error){
		"delivery": func() ([]byte, error) { return wire.DeliveryFrame{Wire: "koine.wire/99"}.MarshalJSON() },
		"yield":    func() ([]byte, error) { return wire.YieldFrame{Wire: "koine.wire/99"}.MarshalJSON() },
		"exchange": func() ([]byte, error) { return wire.ExchangeFrame{Wire: "koine.wire/99"}.MarshalJSON() },
		"opened":   func() ([]byte, error) { return wire.OpenedFrame{Wire: "koine.wire/99"}.MarshalJSON() },
		"ack":      func() ([]byte, error) { return wire.AckFrame{Wire: "koine.wire/99"}.MarshalJSON() },
		"value":    func() ([]byte, error) { return wire.ValueFrame{Wire: "koine.wire/99"}.MarshalJSON() },
	}
	decoders := map[string]func([]byte) error{
		"delivery": func(b []byte) error { _, err := wire.DecodeDelivery(b); return err },
		"yield":    func(b []byte) error { _, err := wire.DecodeYield(b); return err },
		"exchange": func(b []byte) error { _, err := wire.DecodeExchange(b); return err },
		"opened":   func(b []byte) error { _, err := wire.DecodeOpened(b); return err },
		"ack":      func(b []byte) error { _, err := wire.DecodeAck(b); return err },
		"value":    func(b []byte) error { _, err := wire.DecodeValue(b); return err },
	}
	for name, render := range foreign {
		t.Run(name, func(t *testing.T) {
			data, err := render()
			if err != nil {
				t.Fatal(err)
			}
			if err := decoders[name](data); !errors.Is(err, wire.ErrVersion) {
				t.Fatalf("a foreign %s frame was read: %v", name, err)
			}
			if err := decoders[name]([]byte(`{}`)); !errors.Is(err, wire.ErrVersion) {
				t.Fatalf("an unversioned %s frame was read: %v", name, err)
			}
		})
	}
}

// TestWire_FramesRoundTripWithoutReflection pins the shape of every frame
// byte for byte. The literals below are the contract as the engine will read
// it, and a change to one of them is a change two trackers gate on — which
// is exactly why they are written out here rather than compared to
// themselves.
func TestWire_FramesRoundTripWithoutReflection(t *testing.T) {
	t.Run("delivery", func(t *testing.T) {
		want := `{"wire":"koine.wire/1","station":"deployment-steward","chain":"chain/7",` +
			`"actor":"sub:mchen;act:conduit","anchor":"deploy","type":"dev.cdevents.deployment.finished",` +
			`"facts":{"subject":"payments-api","outcome":"failure"}}`
		f := wire.DeliveryFrame{
			Wire: wire.Version, Station: "deployment-steward", Chain: "chain/7",
			Actor: "sub:mchen;act:conduit", Anchor: "deploy",
			Type:  "dev.cdevents.deployment.finished",
			Facts: []byte(`{"subject":"payments-api","outcome":"failure"}`),
		}
		data := render(t, f)
		if data != want {
			t.Fatalf("wrote\n  %s\nwant\n  %s", data, want)
		}
		back, err := wire.DecodeDelivery([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		if back.Station != f.Station || back.Chain != f.Chain || back.Actor != f.Actor ||
			back.Anchor != f.Anchor || back.Type != f.Type || string(back.Facts) != string(f.Facts) {
			t.Fatalf("round trip lost keys: %#v", back)
		}
	})

	t.Run("yield", func(t *testing.T) {
		want := `{"wire":"koine.wire/1","type":"dev.cdevents.deployment.requested","body":{"artifact":"sha256:good","target":"prod"}}`
		f := wire.YieldFrame{
			Wire: wire.Version, Type: "dev.cdevents.deployment.requested",
			Body: []byte(`{"artifact":"sha256:good","target":"prod"}`),
		}
		if data := render(t, f); data != want {
			t.Fatalf("wrote\n  %s\nwant\n  %s", data, want)
		}
		back, err := wire.DecodeYield([]byte(want))
		if err != nil {
			t.Fatal(err)
		}
		if back.Type != f.Type || string(back.Body) != string(f.Body) {
			t.Fatalf("round trip lost keys: %#v", back)
		}
	})

	t.Run("exchange", func(t *testing.T) {
		want := `{"wire":"koine.wire/1","seat":"history","name":"history.last",` +
			`"args":[{"name":"outcome","value":"success"}]}`
		f := wire.ExchangeFrame{
			Wire: wire.Version, Seat: "history", Name: "history.last",
			Args: []koine.Arg{{Name: "outcome", Value: "success"}},
		}
		if data := render(t, f); data != want {
			t.Fatalf("wrote\n  %s\nwant\n  %s", data, want)
		}
		back, err := wire.DecodeExchange([]byte(want))
		if err != nil {
			t.Fatal(err)
		}
		if back.Seat != f.Seat || back.Name != f.Name || len(back.Args) != 1 ||
			back.Args[0] != f.Args[0] {
			t.Fatalf("round trip lost keys: %#v", back)
		}

		// An intent with no arguments still writes the key, so the shape
		// of a frame never depends on what happened to be in it.
		empty := wire.ExchangeFrame{Wire: wire.Version, Seat: "ledger", Name: "ledger.note"}
		if data := render(t, empty); !strings.Contains(data, `"args":[]`) {
			t.Errorf("an argument-less intent wrote %s", data)
		}
	})

	t.Run("opened", func(t *testing.T) {
		want := `{"wire":"koine.wire/1","token":18446744073709551615,"err":""}`
		f := wire.OpenedFrame{Wire: wire.Version, Token: 1<<64 - 1}
		if data := render(t, f); data != want {
			t.Fatalf("wrote\n  %s\nwant\n  %s", data, want)
		}
		back, err := wire.DecodeOpened([]byte(want))
		if err != nil {
			t.Fatal(err)
		}
		if back.Token != f.Token {
			t.Fatalf("a token must survive its whole range: %d", back.Token)
		}
	})

	t.Run("ack", func(t *testing.T) {
		want := `{"wire":"koine.wire/1","state":"received","by":"sub:history-fulfiller"}`
		f := wire.AckFrame{Wire: wire.Version, State: wire.StateReceived, By: "sub:history-fulfiller"}
		if data := render(t, f); data != want {
			t.Fatalf("wrote\n  %s\nwant\n  %s", data, want)
		}
		back, err := wire.DecodeAck([]byte(want))
		if err != nil {
			t.Fatal(err)
		}
		if back.State != f.State || back.By != f.By {
			t.Fatalf("round trip lost keys: %#v", back)
		}
	})

	t.Run("value", func(t *testing.T) {
		want := `{"wire":"koine.wire/1","state":"filled","by":"sub:history-fulfiller",` +
			`"body":{"artifactId":"sha256:good"},"err":""}`
		f := wire.ValueFrame{
			Wire: wire.Version, State: wire.StateFilled, By: "sub:history-fulfiller",
			Body: []byte(`{"artifactId":"sha256:good"}`),
		}
		if data := render(t, f); data != want {
			t.Fatalf("wrote\n  %s\nwant\n  %s", data, want)
		}
		back, err := wire.DecodeValue([]byte(want))
		if err != nil {
			t.Fatal(err)
		}
		if back.State != f.State || string(back.Body) != string(f.Body) {
			t.Fatalf("round trip lost keys: %#v", back)
		}

		// A frame with no payload writes null and reads back as nothing,
		// which is what an absent projection means.
		breached := wire.ValueFrame{Wire: wire.Version, State: wire.StateBreached, Err: "no-such-deployment"}
		data := render(t, breached)
		if !strings.Contains(data, `"body":null`) {
			t.Errorf("an empty payload wrote %s", data)
		}
		back, err = wire.DecodeValue([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		if back.Body != nil {
			t.Errorf("null read back as %q", back.Body)
		}
		if back.Err != "no-such-deployment" {
			t.Errorf("the outcome variant did not survive: %q", back.Err)
		}
	})
}

// TestWire_FramesCarryPayloadsWithoutParsingThem pins the division of
// labour: a stratum's shape belongs to the stratum, and the wire moves it
// whole. This is also why a delivery may grow keys while K2 is in flight
// without moving Version — the wire carries whatever it is handed.
func TestWire_FramesCarryPayloadsWithoutParsingThem(t *testing.T) {
	odd := `{"a":[1,{"b":"}"},null],"deep":{"deeper":{"deepest":[]}},"n":-1.5e3}`
	f := wire.DeliveryFrame{Wire: wire.Version, Type: "x", Facts: []byte(odd)}
	data := render(t, f)
	back, err := wire.DecodeDelivery([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if string(back.Facts) != odd {
		t.Fatalf("the payload came back as\n  %s\nwant\n  %s", back.Facts, odd)
	}
}

// TestWire_UnreadableFramesAreRefusedByName keeps the contract on the same
// posture as every other door in this repository: refuse, and say where.
func TestWire_UnreadableFramesAreRefusedByName(t *testing.T) {
	for _, bad := range []string{
		``,
		`{`,
		`{"wire":}`,
		`{"wire":"koine.wire/1"} trailing`,
		`[1,2]`,
		`{"wire":"koine.wire/1","facts":}`,
	} {
		if _, err := wire.DecodeDelivery([]byte(bad)); err == nil {
			t.Errorf("%q was read", bad)
		}
	}
	if _, err := wire.DecodeOpened([]byte(`{"wire":"koine.wire/1","token":"seven"}`)); err == nil {
		t.Error("a token that is not a number was read")
	}
	if _, err := wire.DecodeExchange([]byte(`{"wire":"koine.wire/1","args":{"name":"x"}}`)); err == nil {
		t.Error("arguments that are not an array were read")
	}
}

// TestWire_ABIIsOneListBothSidesRead pins that the names on the boundary are
// DATA. The engine's loader gates on this package's lists; if either side
// kept its own copy they would drift, and the drift would surface as a load
// failure nobody could explain.
func TestWire_ABIIsOneListBothSidesRead(t *testing.T) {
	if wire.Module != "koine" {
		t.Errorf("the import module is %q", wire.Module)
	}
	wantImports := []string{"ack_poll", "exchange", "value_poll", "yield"}
	if strings.Join(wire.GuestImports, ",") != strings.Join(wantImports, ",") {
		t.Errorf("guest imports = %v, want %v", wire.GuestImports, wantImports)
	}
	wantExports := []string{"deliver", "inbox", "manifest"}
	if strings.Join(wire.GuestExports, ",") != strings.Join(wantExports, ",") {
		t.Errorf("guest exports = %v, want %v", wire.GuestExports, wantExports)
	}
	if len(wire.GuestImports) != 4 {
		t.Errorf("DESIGN §8 names five host functions, four of them guest→host; this build imports %d", len(wire.GuestImports))
	}

	// The lists are sorted, because a set that is compared by joining is
	// a set that has to have an order.
	for _, list := range [][]string{wire.GuestImports, wire.GuestExports, wire.ToolchainExports} {
		for i := 1; i < len(list); i++ {
			if list[i-1] >= list[i] {
				t.Errorf("%v is not sorted", list)
				break
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
// deliver export, including the one that is deliberately absent.
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
	if wire.Resolved != 0 {
		t.Error("resolved must be zero: a guest that returns nothing returned normally")
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
