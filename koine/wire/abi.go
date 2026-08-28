package wire

// The ABI: the names on the boundary and the one number-shaped convention
// that crosses it. All of it is data, so the engine's loader and this guest
// gate on the same list rather than on two copies of the same list.

// Module is the wasm import module every guest→host call is imported from.
// One module, four functions, and a reader can count them.
const Module = "koine"

// The guest→host imports (DESIGN §8).
const (
	ImportYield     = "yield"
	ImportExchange  = "exchange"
	ImportAckPoll   = "ack_poll"
	ImportValuePoll = "value_poll"
)

// The guest's own exports.
const (
	// ExportManifest returns the packed address and length of the
	// station's manifest JSON. This is the engine's existing manifest
	// door (A2): the same export name, read once at load, bounded, and
	// validated before any guest executes.
	ExportManifest = "manifest"
	// ExportDeliver takes the length of a delivery frame already written
	// into the inbox and returns an Outcome.
	ExportDeliver = "deliver"
	// ExportInbox returns the packed address and capacity of the buffer
	// the host writes frames into. The host reads it once and never
	// assumes a size.
	ExportInbox = "inbox"
)

// GuestImports is the closed set of host functions a conforming guest may
// import. A guest importing anything else is reaching for a channel that
// does not exist, and the loader refuses it by name.
var GuestImports = []string{ImportAckPoll, ImportExchange, ImportValuePoll, ImportYield}

// GuestExports is the closed set of exports a conforming guest declares.
var GuestExports = []string{ExportDeliver, ExportInbox, ExportManifest}

// ToolchainExports are the names the wasm toolchain emits for EVERY module
// it builds, conforming or not. They are not the guest's declarations and
// they are not a second path: memory is the linear memory the host must
// reach to write the inbox at all, _initialize is the module initializer the
// host calls before anything else, and the four float helpers are
// compiler-rt intrinsics that an empty main() emits just as readily as a
// station does.
//
// This list exists so the export check can be exact instead of approximate.
// DESIGN §13's K2 done-condition says a guest exports "the manifest plus the
// wire entry points — nothing else"; taken literally that is unreachable on
// any real toolchain, and pretending otherwise would leave the check either
// permanently red or quietly loosened. Naming the toolchain's own set here
// keeps the claim exact and keeps the two sides gating on one list. Verified
// against TinyGo 0.41.1, target wasm-unknown, by building an empty main.
var ToolchainExports = []string{
	"_initialize",
	"fmaximum",
	"fmaximumf",
	"fminimum",
	"fminimumf",
	"memory",
}

// Outcome is what deliver answers with. A trap is not in this list because a
// trap does not return: the host observes it and attributes it to
// work.finished{outcome: failure} (§8).
type Outcome uint32

const (
	// Resolved: the body ran to completion.
	Resolved Outcome = 0
	// Cancelled: a yield was refused, so the host cancelled and nothing
	// after the refusal was spoken.
	Cancelled Outcome = 1
	// Refused: the frame could not be read — a foreign version, a
	// malformed projection, a station this guest does not serve. Nothing
	// ran, and nothing is stored on refusal (A9).
	Refused Outcome = 2
)

// String names an outcome for a person reading a log.
func (o Outcome) String() string {
	switch o {
	case Resolved:
		return "resolved"
	case Cancelled:
		return "cancelled"
	case Refused:
		return "refused"
	}
	return "unknown"
}

// InboxCapacity is the size of the buffer the host writes frames into. The
// host reads the real capacity from the inbox export rather than trusting
// this constant — a guest built against a later SDK may carry a different
// one, and the export is the truth.
const InboxCapacity = 64 << 10

// Pack folds an address and a length into the single i64 a wasm export can
// return. High half address, low half length.
func Pack(addr, length uint32) uint64 {
	return uint64(addr)<<32 | uint64(length)
}

// Unpack splits what Pack folded.
func Unpack(packed uint64) (addr, length uint32) {
	return uint32(packed >> 32), uint32(packed)
}
