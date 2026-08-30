// Command chainhooks is the HOOK form of the pass-up compiled to a wasm
// guest: a station that writes no verb at all and declares Pre and Post.
//
// It exists so hook traffic reaches the real boundary at least once. The
// named hooks are sugar over the three verbs and nothing else, and a claim
// like that is worth exactly as much as the place it is checked — off-target
// the twins are compared frame for frame, and here the sugar is put in front
// of the loader that will one day read it.
//
// It is the SDK half of koine-go#5, and its counterpart is conduit-go#210's
// ChainBroker, which does not exist yet. So what this guest proves today is
// bounded and worth stating: it LOADS into the real koinehost, its manifest
// reads, and the frames it speaks are well formed. It does not prove
// conformance, because there is nothing on the other side to conform to. See
// koine/wire/passup.go's PROPOSED SPELLINGS block.
//
// Build it the way the engine does:
//
//	tinygo build -o chainhooks.wasm -target wasm-unknown ./fixtures/guest/chainhooks
package main

import (
	_ "embed"

	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/station"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/wire"
)

// manifestJSON is DERIVED, never written here (A3). koinegen extracts it
// from the station's own body; this file only carries it across the
// boundary. Regenerate with `go generate ./fixtures/guest/chainhooks`.
//
//go:generate go run github.com/sol-duara-inc/koine-go/cmd/koinegen manifest -registry ../../../cmd/koinegen/testdata/registry -station ../../../cmd/koinegen/fixtures/station -koine ChainHooks -o manifest.json
//go:embed manifest.json
var manifestJSON []byte

// guest is the module's one station. A wasm module is a singleton by nature
// and so is this: one station, one host below it, one inbox.
//
// The station is a POINTER because construction is delivery — the host fills
// its chain and its actor before Resolve, and standard parts written into a
// copy are parts nobody can read.
var guest = wire.Serve(wire.Station{
	Koine:    &station.ChainHooks{},
	Manifest: manifestJSON,
	Decode:   decode,
})

// decode reads the projected facts into this station's stratum. It is three
// lines, and it is the one thing the wire cannot know: the delivery type
// belongs to a stratum, and the wire has no business knowing a stratum's
// shape. The host has already cut the JSON to this station's lineage, so
// there is nothing here to filter — two walls, one semantic.
func decode(f wire.DeliveryFrame) (koine.Delivery, error) {
	var d deployment.ResolvedDelivery
	if err := d.UnmarshalJSON(f.Event); err != nil {
		return nil, err
	}
	return d, nil
}
