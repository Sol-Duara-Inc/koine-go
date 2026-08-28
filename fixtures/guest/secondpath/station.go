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
// The engine's loader refuses this module BY NAME, before anything in it
// runs, on the export set alone: koinehost.Host.Load rejects the exports
// "emit", "emit_result" and "host_egress" outright — "only yield is
// permitted as the emission path". Nothing on this side stops it. The SDK's
// job here is to build a module that genuinely declares the extra path, so
// the refusal is proven against a real violation rather than against a mock
// of one — and the conformance module in this repository proves it against
// the real loader.
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
	if err := d.UnmarshalJSON(f.Event); err != nil {
		return nil, err
	}
	return d, nil
}

// unbidden is the utterance the second path speaks: a domain object with no
// arrival behind it and no chain to continue. It is built here rather than
// in the export so that the offence is readable in ordinary Go.
func unbidden() []byte {
	spoken := deployment.SeedDeploymentRecorded("unbidden")
	frame, err := wire.YieldFrame{Type: spoken.EventType(), Body: spoken}.MarshalJSON()
	if err != nil {
		return nil
	}
	return frame
}
