package wire

// The ABI: the names on the boundary, their signatures, and the one
// number-shaped convention that crosses it. All of it is data, so the
// engine's loader and this guest gate on the same list rather than on two
// copies of the same list.
//
// EVERY NAME AND SIGNATURE BELOW IS THE HOST'S. conduit-go's pkg/koinehost
// is normative (ruled 2026-08-28, koine-go#12): the host defines and the
// guest conforms. What is written here was read off the merged loader —
// Host.Load's requireSig calls and NewHost's host module builder — and the
// conformance module in this repository proves it against the real thing
// rather than against this file's opinion of it.

// Module is the wasm import module the host registers. It registers a second
// name, Alias, for the same functions; a guest may import from either and
// the loader refuses any other module.
const (
	Module = "koine"
	Alias  = "conduit"
)

// The guest→host imports. The host registers all six; a guest imports the
// ones it reaches, and the linker drops the rest.
const (
	// ImportDeliver pulls the delivery the host is holding: (i32,i32)->i64,
	// answering a packed pointer and length into guest memory the host
	// allocated through the alloc export. The push path (ExportResolve's
	// own arguments) is what this SDK uses; the pull path is the host's
	// and is named here for completeness.
	ImportDeliver = "deliver"
	// ImportYield is the guest's ONLY emit path: (i32,i32)->i32, answering
	// ZERO for success and non-zero for a host that could not take it.
	ImportYield = "yield"
	// ImportExchange utters an intent: (i32,i32)->i64, answering the
	// handle the broker minted, or zero for a refusal.
	ImportExchange = "exchange"
	// ImportAckPoll is the fast beat: (i64)->i32, answering non-zero once
	// the broker has the intent.
	ImportAckPoll = "ack_poll"
	// ImportValuePoll waits until the exchange is filled or breached:
	// (i64)->i64, answering a packed pointer and length into guest memory
	// the host allocated through the alloc export.
	ImportValuePoll = "value_poll"
	// ImportHostLog carries a diagnostic line: (i32,i32)->(). It is not an
	// emit path — the host records it beside the run and never stores it
	// as an event.
	ImportHostLog = "host_log"
)

// The guest's own exports.
const (
	// ExportAlloc hands the host a place to write: (i32)->i32. The host
	// calls it before every frame it pushes — the delivery, and every
	// exchange answer — and writes at the address it answers with. Guest
	// memory belongs to the guest, and this is the only door into it.
	ExportAlloc = "alloc"
	// ExportResolve is the delivery: (i32,i32)->i32, taking the pointer
	// and length of a delivery frame the host has already written, and
	// answering an Outcome. Zero is success; the host reads anything else
	// as work.finished{outcome: failure}.
	ExportResolve = "resolve"
	// ExportManifest carries the station's declaration: ()->i64, a packed
	// pointer and length. The host reads it ONCE at load, bounded by
	// MaxManifestBytes, and validates it before any guest executes (A2).
	ExportManifest = "manifest"
)

// GuestImports is the closed set of host functions a conforming guest may
// import. A guest importing anything else is reaching for a channel that
// does not exist, and the loader refuses it by name.
var GuestImports = []string{
	ImportAckPoll, ImportDeliver, ImportExchange, ImportHostLog, ImportValuePoll, ImportYield,
}

// GuestExports is the closed set of exports a conforming guest declares.
var GuestExports = []string{ExportAlloc, ExportManifest, ExportResolve}

// ForbiddenExports are the names the loader refuses BY NAME, whatever else
// the module looks like: they are second emit paths, and yield is the only
// one. The negative fixture in this repository declares one of them on
// purpose, so the refusal is proven against a real offence.
var ForbiddenExports = []string{"emit", "emit_result", "host_egress"}

// ForbiddenImports are the host functions no guest may reach for. They do
// not exist in the host module; naming them here is what lets the loader
// refuse a guest that asks by name rather than by absence.
var ForbiddenImports = []string{"emit_result", "host_egress"}

// ToolchainExports are the names the wasm toolchain emits for EVERY module
// it builds, conforming or not. They are not the guest's declarations and
// they are not a second path: memory is the linear memory the host must
// reach to write into the guest at all, _initialize is the module
// initializer the host calls before anything else, and the four float
// helpers are compiler-rt intrinsics that an empty main() emits just as
// readily as a station does.
//
// This list exists so the export check can be exact instead of approximate.
// DESIGN §13's K2 done-condition says a guest exports "the manifest plus the
// wire entry points — nothing else"; taken literally that is unreachable on
// any real toolchain, and pretending otherwise would leave the check either
// permanently red or quietly loosened. Verified against TinyGo 0.41.1,
// target wasm-unknown, by building an empty main.
var ToolchainExports = []string{
	"_initialize",
	"fmaximum",
	"fmaximumf",
	"fminimum",
	"fminimumf",
	"memory",
}

// MaxManifestBytes is the bound the host reads the manifest under. A guest
// that would export more is refused at load, so the bound is the guest's to
// respect too.
const MaxManifestBytes = 64 << 10

// Outcome is what resolve answers with. The host reads ZERO as success and
// everything else as work.finished{outcome: failure} with the status
// attributed.
//
// A trap is not in this list because a trap does not return — and that is
// the whole point of the list. koinehost records a trap as "trap in
// resolve", which names the AUTHOR. Every non-zero value below names
// something else: the host could not take an utterance, could not answer an
// exchange, or handed over a frame this build cannot read. None of those are
// the body's doing, so none of them trap, and the number the host reads says
// which one happened. The words behind it go through host_log with
// FaultPrefix in front of them and arrive in koinehost's Result.Logs.
//
// The engine still records all of these as work.finished{outcome: failure},
// because that is the only terminal disposition wire v1 gives it. A
// disposition that says "the fabric stopped this, not the station" is
// conduit-go's to add; what this SDK can do is stop claiming the author did
// it, and it does.
type Outcome uint32

const (
	// Resolved: the body ran to completion. The author's.
	Resolved Outcome = 0
	// Cancelled: the host would not take a yield, so nothing after the
	// refusal was spoken. Below the line.
	Cancelled Outcome = 1
	// Refused: the frame could not be read — a foreign version, a
	// malformed projection, a station this guest does not serve. Nothing
	// ran, and nothing is stored on refusal (A9). Below the line.
	Refused Outcome = 2
	// Unanswered: an exchange the body spoke stopped for a reason below
	// the line — no broker wired, no handle, no answer, or an answer that
	// was neither filled nor breached. The body may have carried on;
	// nothing it said afterwards was stored. Below the line.
	Unanswered Outcome = 3
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
	case Unanswered:
		return "unanswered"
	}
	return "unknown"
}

// AttributedToAuthor reports whether an outcome is the station author's to
// answer for. Only a completed run is — everything else this type can say
// happened below the line, and a trap, which is the author's, is not
// representable here because it never returns.
func (o Outcome) AttributedToAuthor() bool { return o == Resolved }

// Pack folds an address and a length into the single i64 that crosses the
// boundary. High half address, low half length — the host's convention,
// read off hostDeliver and hostValuePoll.
func Pack(addr, length uint32) uint64 {
	return uint64(addr)<<32 | uint64(length)
}

// Unpack splits what Pack folded.
func Unpack(packed uint64) (addr, length uint32) {
	return uint32(packed >> 32), uint32(packed)
}
