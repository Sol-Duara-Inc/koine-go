//go:build !wasm && !tinygo.wasm

package wire

// Off-target, the four host calls do not exist — there is no module below
// this build to import them from. They are not stubbed into something
// plausible: a guest that finds itself outside the sandbox says so, once,
// rather than answering a delivery with an invention.
//
// This file exists so the package builds, vets and tests everywhere. The
// guest runtime in guest.go is ordinary Go and is tested here against a
// scripted Host; only the four imports and the three export bodies are
// wasm-only, and they are the thinnest part of the package on purpose.

type sandbox struct{ g *Guest }

// Sandbox is the Host a guest uses in the engine. Off-target it answers
// nothing: every call panics with ErrNoHost's sentence.
func Sandbox(g *Guest) Host { return sandbox{g: g} }

func (s sandbox) Yield([]byte) bool       { panic(ErrNoHost.Error()) }
func (s sandbox) Exchange([]byte) []byte  { panic(ErrNoHost.Error()) }
func (s sandbox) AckPoll(uint64) []byte   { panic(ErrNoHost.Error()) }
func (s sandbox) ValuePoll(uint64) []byte { panic(ErrNoHost.Error()) }

// ManifestExport is meaningful only inside a wasm module: a guest address is
// 32 bits and this build's are not.
func (g *Guest) ManifestExport() uint64 { panic(ErrNoHost.Error()) }

// InboxExport is meaningful only inside a wasm module.
func (g *Guest) InboxExport() uint64 { panic(ErrNoHost.Error()) }

// DeliverExport is meaningful only inside a wasm module. The portable core
// it wraps is Deliver, which runs anywhere.
func (g *Guest) DeliverExport(uint32) uint32 { panic(ErrNoHost.Error()) }
