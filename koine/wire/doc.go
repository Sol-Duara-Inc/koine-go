// Package wire is the versioned guest↔host contract: the ONLY coupling
// between this SDK and the engine that runs its stations.
//
// The engine vendors nothing from this repository but this package's shape,
// and this repository imports nothing of the engine's at all. That is the
// whole treaty. Everything else on either side may move without asking; a
// change here is a change to a contract two trackers gate on, which is why
// Version exists and why a frame that names a version this build does not
// speak is refused BY NAME rather than parsed hopefully.
//
// # The five host functions
//
// DESIGN §8 names the entire guest-visible surface, and it is five calls.
// Four are guest→host (the guest imports them from module "koine"):
//
//	yield        the guest speaks one utterance; the host forms the
//	             envelope, continues the chain, and stores by emitting
//	exchange     the guest utters an intent at a seat; the host opens the
//	             exchange and answers with a token
//	ack_poll     the fast beat: has anyone who declared comprehension
//	             received this yet
//	value_poll   is the exchange filled, breached, or still pending
//
// One is host→guest, and it is the guest's own export:
//
//	deliver      the host hands over a projected delivery and the
//	             construction context; the guest resolves and yields
//
// There is no sixth. The guest has no emit path but yield — not because
// anything guards the others, but because there are no others: a wasm module
// can only call what it imports, and this package declares what it imports
// in one place, in source, where a reader can count them.
//
// # Waiting
//
// Value() waits until the exchange is filled or breached (E-C, amended
// 2026-08-27). The guest renders that wait as a poll loop over value_poll,
// which is deliberately the dumbest possible guest: it lets the HOST choose
// the mechanism. A host that suspends and resumes the guest across the
// boundary and a host that simply does not return until there is news are
// both conforming, and neither is a fact this package pins.
//
// # Memory
//
// Guest linear memory belongs to the guest. Frames the guest sends are
// passed as a pointer and a length into memory the guest already owns;
// frames the host sends are written into the guest's inbox, whose address
// and capacity the host reads once from the inbox export. The host never
// allocates in guest memory and the guest never hands out a pointer it does
// not own. Nothing here needs a garbage collector to agree with anything.
//
// # Reflection-free
//
// Every frame marshals and unmarshals through koine/codec, naming each key
// in source (A6). The guest target stays open, and the whole contract is
// readable as JSON by a person debugging a load failure at three in the
// morning.
package wire
