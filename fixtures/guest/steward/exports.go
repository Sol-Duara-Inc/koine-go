//go:build tinygo.wasm

// tinygo.wasm, not wasm: TinyGo's wasm-unknown target does not set the wasm
// tag (it builds the standard library as GOARCH=arm), and standard Go's
// //go:wasmexport needs -buildmode=c-shared, which a guest is not. The
// exports are TinyGo's to declare; everything else in this SDK builds under
// both toolchains.

package main

// The guest's whole declared surface: three exports, and this file is all of
// them. A reader can count them, and so can the engine's loader — the names
// are koine/wire's GuestExports, and neither side keeps its own copy.

//go:wasmexport manifest
func manifestExport() uint64 { return guest.ManifestExport() }

//go:wasmexport inbox
func inboxExport() uint64 { return guest.InboxExport() }

//go:wasmexport deliver
func deliverExport(length uint32) uint32 { return guest.DeliverExport(length) }

// main is required and does nothing. A station is not a program: it stands
// still and speaks when it is delivered to.
func main() {}
