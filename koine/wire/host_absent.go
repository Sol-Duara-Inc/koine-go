//go:build !wasm && !tinygo.wasm

package wire

// Off-target, the host calls do not exist — there is no module below this
// build to import them from. They are not stubbed into something plausible:
// a guest that finds itself outside the sandbox says so, once, rather than
// answering a delivery with an invention.
//
// This file exists so the package builds, vets and tests everywhere. The
// guest runtime in guest.go is ordinary Go and is tested here against a
// scripted Host; only the six imports and the three export bodies are
// wasm-only, and they are the thinnest part of the package on purpose —
// everything they do is proven inside the real sandbox by the conformance
// module, against the real loader.

type sandbox struct{ g *Guest }

// Sandbox is the Host a guest uses in the engine. Off-target it answers
// nothing: every call panics with ErrNoHost's sentence.
func Sandbox(g *Guest) Host { return sandbox{g: g} }

func (s sandbox) Yield([]byte) uint32     { panic(ErrNoHost.Error()) }
func (s sandbox) Exchange([]byte) uint64  { panic(ErrNoHost.Error()) }
func (s sandbox) AckPoll(uint64) uint32   { panic(ErrNoHost.Error()) }
func (s sandbox) ValuePoll(uint64) []byte { panic(ErrNoHost.Error()) }
func (s sandbox) Log(string)              { panic(ErrNoHost.Error()) }

// AllocExport is meaningful only inside a wasm module: a guest address is 32
// bits and this build's are not. Under the sandbox it traps when the arena
// cannot serve, because alloc has no failure channel and address zero is a
// real address the host would happily write over.
func (g *Guest) AllocExport(uint32) uint32 { panic(ErrNoHost.Error()) }

// ManifestExport is meaningful only inside a wasm module.
func (g *Guest) ManifestExport() uint64 { panic(ErrNoHost.Error()) }

// ResolveExport is meaningful only inside a wasm module. The portable core
// it wraps is Deliver, which runs anywhere.
func (g *Guest) ResolveExport(uint32, uint32) uint32 { panic(ErrNoHost.Error()) }
