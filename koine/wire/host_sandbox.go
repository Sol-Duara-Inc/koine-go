//go:build wasm || tinygo.wasm

// Two tags, and a filename that carries neither of them, because the two
// supported toolchains spell the same fact differently (E-A: both are
// supported build targets):
//
//   - Standard Go sets wasm from GOARCH.
//   - TinyGo's wasm-unknown target does NOT. It builds the standard library
//     as GOARCH=arm and marks the wasm-ness with tinygo.wasm instead —
//     `tinygo info -target wasm-unknown` on 0.41.1 lists the tags, and wasm
//     is not among them.
//
// The filename matters as much as the constraint. A file named *_wasm.go
// carries an IMPLICIT GOARCH=wasm constraint that no //go:build line can
// widen, so under TinyGo it would be dropped by its name before the line
// above was ever read — and the guest would ship with no host bindings at
// all, silently, having compiled clean. Hence host_sandbox.go.

package wire

import "unsafe"

// The four guest→host imports, and nothing else. This is the entire list;
// count them. A wasm module can only call what it imports, so the claim that
// the guest has no emit path but yield is not enforced by a guard — it is
// enforced by the absence of an import to reach for.
//
// Every one of them takes a pointer and a length into memory the GUEST owns,
// and answers with a packed pointer and length into the guest's inbox, which
// the guest also owns. The host never allocates in guest memory.

//go:wasmimport koine yield
func hostYield(addr, length uint32) uint32

//go:wasmimport koine exchange
func hostExchange(addr, length uint32) uint64

//go:wasmimport koine ack_poll
func hostAckPoll(token uint64) uint64

//go:wasmimport koine value_poll
func hostValuePoll(token uint64) uint64

// sandbox is the Host below a guest actually running in the engine.
type sandbox struct{ g *Guest }

// Sandbox is the Host a guest uses in the engine. It is the only Host this
// SDK ships that reaches a real engine, and it is reachable only from inside
// a wasm module.
func Sandbox(g *Guest) Host { return sandbox{g: g} }

func (s sandbox) Yield(frame []byte) bool {
	addr, length := span(frame)
	return hostYield(addr, length) != 0
}

func (s sandbox) Exchange(frame []byte) []byte {
	addr, length := span(frame)
	return s.g.read(hostExchange(addr, length))
}

func (s sandbox) AckPoll(token uint64) []byte { return s.g.read(hostAckPoll(token)) }

func (s sandbox) ValuePoll(token uint64) []byte { return s.g.read(hostValuePoll(token)) }

// read takes a packed address and length the host just wrote into the inbox
// and returns those bytes. The slice is the inbox itself: the caller decodes
// before the next host call overwrites it, which every caller here does.
func (g *Guest) read(packed uint64) []byte {
	addr, length := Unpack(packed)
	inbox, _ := span(g.inbox)
	if addr != inbox || uint64(length) > uint64(len(g.inbox)) {
		panic("koine/wire: the host answered with bytes outside this guest's inbox")
	}
	return g.inbox[:length]
}

// span is the address and length of a slice in guest linear memory. Guest
// memory is 32-bit, so the truncation is the address, not a loss of one.
func span(b []byte) (addr, length uint32) {
	if len(b) == 0 {
		return 0, 0
	}
	return uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b))
}

// ManifestExport is the body of the manifest export: the packed address and
// length of the derived manifest JSON, read once by the host at load.
func (g *Guest) ManifestExport() uint64 {
	addr, length := span(g.station.Manifest)
	return Pack(addr, length)
}

// InboxExport is the body of the inbox export: the packed address and
// capacity of the buffer the host writes frames into. The host reads it once
// and never assumes a size.
func (g *Guest) InboxExport() uint64 {
	addr, _ := span(g.inbox)
	return Pack(addr, uint32(len(g.inbox)))
}

// DeliverExport is the body of the deliver export: the host has written a
// delivery frame of the given length into the inbox.
func (g *Guest) DeliverExport(length uint32) uint32 {
	if uint64(length) > uint64(len(g.inbox)) {
		return uint32(g.refuse("the host wrote a frame larger than this guest's inbox"))
	}
	return uint32(g.Deliver(g.inbox[:length]))
}
