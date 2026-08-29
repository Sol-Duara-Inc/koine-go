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

import (
	"strconv"
	"unsafe"
)

// itoa keeps the trap message readable without pulling fmt into a guest.
func itoa(n int) string { return strconv.Itoa(n) }

// The guest→host imports, and nothing else. This is the entire list; count
// them. A wasm module can only call what it imports, so the claim that the
// guest has no emit path but yield is not enforced by a guard — it is
// enforced by the absence of an import to reach for.
//
// Every signature below is the host's, read off the functions NewHost
// registers on its "koine" module.

//go:wasmimport koine yield
func hostYield(addr, length uint32) uint32

//go:wasmimport koine exchange
func hostExchange(addr, length uint32) uint64

//go:wasmimport koine ack_poll
func hostAckPoll(handle uint64) uint32

//go:wasmimport koine value_poll
func hostValuePoll(handle uint64) uint64

//go:wasmimport koine host_log
func hostLog(addr, length uint32)

// sandbox is the Host below a guest actually running in the engine.
type sandbox struct{ g *Guest }

// Sandbox is the Host a guest uses in the engine. It is the only Host this
// SDK ships that reaches a real engine, and it is reachable only from inside
// a wasm module.
func Sandbox(g *Guest) Host { return sandbox{g: g} }

func (s sandbox) Yield(frame []byte) uint32 {
	addr, length := span(frame)
	return hostYield(addr, length)
}

func (s sandbox) Exchange(frame []byte) uint64 {
	addr, length := span(frame)
	return hostExchange(addr, length)
}

func (s sandbox) AckPoll(handle uint64) uint32 { return hostAckPoll(handle) }

func (s sandbox) ValuePoll(handle uint64) []byte { return s.g.read(hostValuePoll(handle)) }

func (s sandbox) Log(msg string) {
	if msg == "" {
		return
	}
	b := []byte(msg)
	addr, length := span(b)
	hostLog(addr, length)
}

// read takes a packed address and length the host wrote through the alloc
// export and returns those bytes. The slice points into the arena; the
// caller decodes before the arena is reset, which every caller here does.
func (g *Guest) read(packed uint64) []byte {
	addr, length := Unpack(packed)
	if addr == 0 || length == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(addr))), length)
}

// span is the address and length of a slice in guest linear memory. Guest
// memory is 32-bit, so the truncation is the address, not a loss of one.
func span(b []byte) (addr, length uint32) {
	if len(b) == 0 {
		return 0, 0
	}
	return uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b))
}

// AllocExport is the body of the alloc export: the host asks for room and
// writes at the address answered.
//
// It TRAPS when the arena cannot serve, and that is deliberate. alloc has no
// failure channel — the host takes whatever address it is given and writes
// there (koinehost host.go: Run writes the delivery at the returned pointer,
// and only reports an error if the write goes out of bounds). Address zero
// is a perfectly valid offset in wasm linear memory, so answering zero would
// have the host quietly writing a delivery over the bottom of this module.
// A trap is the only refusal this door has, and the host attributes it as
// work.finished{outcome: failure} with the reason attached.
func (g *Guest) AllocExport(size uint32) uint32 {
	b := g.arena.alloc(int(size))
	if b == nil {
		panic("koine/wire: the host asked for " + itoa(int(size)) +
			" bytes and this guest's arena holds " + itoa(ArenaCapacity) +
			" — answering an address it does not own would have the host write over this module")
	}
	if size == 0 {
		// A zero-length slice has no first element to address; hand back
		// the arena's own base, which is a real address the host will
		// write nothing at.
		addr, _ := span(g.arena.buf)
		return addr
	}
	addr, _ := span(b)
	return addr
}

// ManifestExport is the body of the manifest export: the packed address and
// length of the derived manifest JSON, read once by the host at load.
func (g *Guest) ManifestExport() uint64 {
	addr, length := span(g.station.Manifest)
	return Pack(addr, length)
}

// ResolveExport is the body of the resolve export: the host has written a
// delivery frame at addr, of the given length, through alloc.
//
// The frame is copied out of the arena before anything else happens, so the
// arena can be reclaimed whole at both ends of the run: once here, freeing
// the delivery the host just wrote, and once on the way out, freeing every
// exchange answer spoken during it.
func (g *Guest) ResolveExport(addr, length uint32) uint32 {
	frame := append([]byte(nil), g.read(Pack(addr, length))...)
	g.arena.reset()
	defer g.arena.reset()
	return uint32(g.Deliver(frame))
}
