// Package wire is the versioned guest↔host contract: the ONLY coupling
// between this SDK and the engine that runs its stations.
//
// # The host is normative
//
// Ruled 2026-08-28 (koine-go#12), after the first attempt at this contract
// shipped two halves that did not meet: conduit-go is the product and
// koine-go is the SDK, so conduit-go's pkg/koinehost DEFINES the wire and
// this package CONFORMS. Every name, signature, JSON key and return
// convention below was read off the merged loader. Nothing here is this
// repository's preference, and where this SDK's earlier design was arguably
// the better one — a guest-owned reusable inbox instead of a per-frame
// allocation — the answer is to propose it as v2 with both sides moving
// together, not to ship it alone.
//
// The conformance module at ../../conformance is what makes that real: it
// builds the committed fixture guests with the pinned toolchain and hands
// them to the real koinehost.Host.Load. Both halves passing their own suites
// is precisely what let them drift; only a test that crosses can catch it.
//
// # The host functions
//
// Six calls, guest→host, imported from the module "koine" (the host
// registers "conduit" as an alias for the same six):
//
//	yield        the guest speaks one utterance; the host forms the
//	             envelope, continues the chain, and stores by emitting.
//	             ZERO is success.
//	exchange     the guest utters an intent at a seat; the host opens the
//	             exchange and answers with a handle. Zero is a refusal.
//	ack_poll     the fast beat: does the broker have this yet
//	value_poll   waits until the exchange is filled or breached, and
//	             answers the response
//	deliver      pulls the delivery the host is holding (the push path —
//	             resolve's own arguments — is what this SDK uses)
//	host_log     a diagnostic line beside the run; not an emit path
//
// And three exports, host→guest:
//
//	alloc        room for the host to write a frame into
//	resolve      one delivery; zero is success
//	manifest     the station's declaration, read once at load
//
// There is no seventh call and no fourth export. A wasm module can only call
// what it imports and can only be called through what it exports, so the
// claim that the guest has no emit path but yield is not enforced by a
// guard — it is enforced by the absence of anything else to reach for. The
// loader refuses the names "emit", "emit_result" and "host_egress" outright,
// and the negative fixture in this repository declares one of them on
// purpose so the refusal is proven against a real offence.
//
// # Waiting
//
// Value() waits until the exchange is filled or breached (E-C, amended
// 2026-08-27). The wait is the HOST'S: value_poll does not return until the
// exchange has gone one way or the other. There is no poll loop here and no
// timeout invented here — a budget is the engine's, minted from the
// manifest's topology, and a guest that made up its own would be a guest
// arguing with the calculus.
//
// # Memory
//
// Guest linear memory belongs to the guest, and alloc is the only door into
// it. The host asks for room before every frame it pushes and writes at the
// address alloc answers with. The allocator is a bump pointer over one fixed
// region, reclaimed whole at both ends of every delivery — see arena.go for
// why that matters even though the host instantiates a fresh module per run.
//
// # Reflection-free
//
// Every frame marshals and unmarshals through koine/codec, naming each key
// in source (A6), in the host's own struct order and honouring the host's
// own omitempty tags. The conformance module compares the two marshallers
// byte for byte, so "the same shape" is a fact rather than an intention.
package wire
