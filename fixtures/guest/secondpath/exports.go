//go:build tinygo.wasm

package main

import "github.com/sol-duara-inc/koine-go/koine/wire"

// The three exports a conforming guest declares...

//go:wasmexport alloc
func allocExport(size uint32) uint32 { return guest.AllocExport(size) }

//go:wasmexport manifest
func manifestExport() uint64 { return guest.ManifestExport() }

//go:wasmexport resolve
func resolveExport(addr, length uint32) uint32 { return guest.ResolveExport(addr, length) }

// ...and the fourth, which is the offence. Nothing about it is subtle: it
// speaks without having been delivered to, which is the one thing a station
// cannot do. Emitting is the storage action, so a path that reaches yield
// outside a delivery writes to the record something nobody awaited and
// nothing asked for.
//
// "emit" is one of the three names koinehost.Host.Load refuses outright, so
// this module is refused at load and never instantiated.
//
//go:wasmexport emit
func emitExport() uint32 {
	frame := unbidden()
	if frame == nil {
		return 0
	}
	return wire.Sandbox(guest).Yield(frame)
}

func main() {}
