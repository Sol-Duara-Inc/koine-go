// Command secondpath is a guest that must be REFUSED at load, and it exists
// to be refused.
//
// It is the §10 steward, byte-identical in body, with one thing added: an
// export named "emit" that speaks an utterance nobody awaited. That is the
// second path — speech with no arrival behind it, reached without a
// delivery, outside the one calling convention. The design's claim is that a
// guest has no emit path but yield; this fixture is what makes that claim
// falsifiable, because a claim nothing can violate is not a claim.
//
// The engine's loader (Sol-Duara-Inc/conduit-go#185) refuses this module by
// NAME, before anything in it runs, on the export set alone — the same door,
// the same reader, and nothing stored on refusal (A9). Nothing on this side
// stops it: the SDK's job here is to build a module that genuinely declares
// the extra path, so the refusal is proven against a real violation rather
// than against a mock of one.
//
// Build it the way the engine does:
//
//	tinygo build -o secondpath.wasm -target wasm-unknown ./fixtures/guest/secondpath
package main

import (
	_ "embed"

	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/station"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/wire"
)

// manifestJSON is the steward's own derived manifest. The manifest is
// HONEST — it declares exactly what the body speaks — which is the point: a
// module can carry a truthful manifest and still declare a path the
// manifest says nothing about, so the loader cannot take the manifest's word
// for the module's surface. It has to read the exports.
//
//go:generate go run github.com/sol-duara-inc/koine-go/cmd/koinegen manifest -registry ../../../cmd/koinegen/testdata/registry -station ../../../cmd/koinegen/fixtures/station -koine DeploymentSteward -o manifest.json
//go:embed manifest.json
var manifestJSON []byte

var guest = wire.Serve(wire.Station{
	Koine:    &station.DeploymentSteward{},
	Manifest: manifestJSON,
	Decode:   decode,
})

func decode(f wire.DeliveryFrame) (koine.Delivery, error) {
	var d deployment.ResolvedDelivery
	if err := d.UnmarshalJSON(f.Facts); err != nil {
		return nil, err
	}
	return d, nil
}

// unbidden is the utterance the second path speaks: a domain object with no
// arrival behind it and no chain to continue. It is built here rather than
// in the export so that the offence is readable in ordinary Go.
func unbidden() []byte {
	spoken := deployment.SeedDeploymentRecorded("unbidden")
	body, err := spoken.MarshalJSON()
	if err != nil {
		return nil
	}
	frame, err := wire.YieldFrame{Wire: wire.Version, Type: spoken.EventType(), Body: body}.MarshalJSON()
	if err != nil {
		return nil
	}
	return frame
}
