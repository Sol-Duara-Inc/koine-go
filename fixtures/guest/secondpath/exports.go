//go:build tinygo.wasm

package main

import "github.com/sol-duara-inc/koine-go/koine/wire"

// The three exports a conforming guest declares...

//go:wasmexport manifest
func manifestExport() uint64 { return guest.ManifestExport() }

//go:wasmexport inbox
func inboxExport() uint64 { return guest.InboxExport() }

//go:wasmexport deliver
func deliverExport(length uint32) uint32 { return guest.DeliverExport(length) }

// ...and the fourth, which is the offence. Nothing about it is subtle: it
// speaks without having been delivered to, which is the one thing a station
// cannot do. Emitting is the storage action, so a path that reaches yield
// outside a delivery writes to the record something nobody awaited and
// nothing asked for.
//
//go:wasmexport emit
func emitExport() uint32 {
	frame := unbidden()
	if frame == nil {
		return 0
	}
	if wire.Sandbox(guest).Yield(frame) {
		return 1
	}
	return 0
}

func main() {}
