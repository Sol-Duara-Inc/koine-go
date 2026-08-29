// Command steward is the DESIGN §10 worked example compiled to a wasm guest:
// the first station to cross the boundary this repository versions.
//
// It is a conformance fixture for the engine's pkg/koinehost
// (Sol-Duara-Inc/conduit-go#185), which loads it, reads its manifest export
// once, hands it a projected delivery, and forms and stores what it yields.
// Nothing here is written for the test — the station body is the same
// DeploymentSteward the manifest extractor reads and the harness drives, in
// the same package, unmodified. That is the whole point: one station body,
// three readers, and the guest is just the third.
//
// Build it the way the engine does:
//
//	tinygo build -o steward.wasm -target wasm-unknown ./fixtures/guest/steward
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
// boundary. Regenerate with `go generate ./fixtures/guest/steward`.
//
//go:generate go run github.com/sol-duara-inc/koine-go/cmd/koinegen manifest -registry ../../../cmd/koinegen/testdata/registry -station ../../../cmd/koinegen/fixtures/station -koine DeploymentSteward -o manifest.json
//go:embed manifest.json
var manifestJSON []byte

// guest is the module's one station. A wasm module is a singleton by nature
// and so is this: one station, one host below it, one inbox.
//
// The station is a POINTER because construction is delivery — the host fills
// its chain and its actor before Resolve, and standard parts written into a
// copy are parts nobody can read.
var guest = wire.Serve(wire.Station{
	Koine:    &station.DeploymentSteward{},
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
