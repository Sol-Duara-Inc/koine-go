// Package manifest is the shape of §7's registration manifest: what a
// station declares to the door that admits it.
//
// The manifest is DERIVED FROM CODE, never hand-written (A3) — koinegen
// extracts it, and a conformance test pins the extractor's output to what
// the station's body actually speaks. This package therefore carries no
// parser for author-supplied manifests and no builder an author could reach:
// it is the vocabulary and the writer, nothing else.
//
// The shape is the ENGINE'S, and the engine is normative (ruled 2026-08-28,
// koine-go#12): conduit-go's koinehost.Manifest defines the keys the loader
// reads at the `manifest` export, and this package writes exactly those,
// with exactly those spellings, above the line.
//
// Below the line — under the "koine" key — ride the Koine fields the loader
// does not read and does not need to: stratum, lineage, the content-hash
// pinned awaits, the consumption topology, the seats and their permissions,
// the declared events with their reconciliation entries. The host unmarshals
// into its own struct and ignores what it does not know, so one document
// serves both readers without either one having to grow a field for the
// other's benefit. A2's "one loader serves both kinds of guest" is what this
// arrangement is for; a superset is how a manifest keeps that promise while
// still saying more than the loader asks.
package manifest

// SchemaVersion names the manifest shape this package writes.
const SchemaVersion = "koine.manifest/1"

// Consumption is how a station consumed a handle it spoke — which IS the
// chain role the exchange takes in the engine's stored expected graph (A4).
//
// A station's own consumption produces exactly two patterns. A third chain
// role exists in the engine's stored expected graph, but it is minted from a
// workflow's own topology declaration — never from a station's Resolve.
// This vocabulary was narrowed to the two below 2026-08-27 (DESIGN.md §6,
// issue #11): a station author was never the one who owned that third fact,
// and naming it here dressed a topology statement up as a statement about
// waiting, which it never was.
type Consumption string

const (
	// Inline: Value() was consumed; the exchange runs in the caller's own
	// sequence, on the same chain.
	Inline Consumption = "inline"
	// Concurrent: the handle was spoken and never consumed; the station's
	// completion gates on the exchange's resolution. The default —
	// nothing is silently ungated.
	Concurrent Consumption = "concurrent"
)

// ChainRole is the engine-side name of the same fact: the two roles a
// station's own consumption can ever produce, out of the three its stored
// expected graph admits (the third is workflow topology — see the
// Consumption doc comment).
func (c Consumption) ChainRole() string {
	switch c {
	case Inline:
		return "main"
	case Concurrent:
		return "blocking"
	}
	return ""
}

// Manifest is one station's registration, derived from its code.
//
// Everything down to Exchanges is koinehost.Manifest, key for key. Koine is
// the extension the loader ignores.
type Manifest struct {
	SchemaVersion string   `json:"schemaVersion"`
	Kind          string   `json:"kind"` // "station"; "plugin" when a tool is served
	Identity      Identity `json:"identity"`
	Claim         string   `json:"claim"`
	Awaits        []string `json:"awaits,omitempty"`
	Complete      string   `json:"complete,omitempty"`
	Emits         []string `json:"emits,omitempty"`
	Exchanges     []string `json:"exchanges,omitempty"`
	Koine         Koine    `json:"koine"`
}

// Identity is the tuple the station carries. The SDK carries the claim and
// never verifies it (A8); the door verifies it against what login already
// established.
type Identity struct {
	Group  string `json:"group,omitempty"`
	Author string `json:"author,omitempty"`
	Name   string `json:"name"`
}

// ClaimPattern is the grammar the loader holds a claim to: a reverse-domain
// name, at least two labels. It is carried here as data so koinegen can
// refuse a claim the loader would refuse, at generation time, rather than
// letting a station find out at load.
//
// Transcribed from koinehost.claimShape. A station's claim is the deepest
// namespace of its lineage — the stratum it speaks from — which is already
// reverse-DNS by the registry's own grammar.
const ClaimPattern = `^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)+$`

// Connection is the shape of connection a plugin station needs — never the
// location. The operator owns the address and the credential; the deployment
// binds the client and hands it in.
type Connection struct {
	Shape       string   `json:"shape"`
	Permissions []string `json:"permissions,omitempty"`
}

// Event is one declared event type with its reconciliation entry — the
// engine family's row. Direction is "emit" for what the station speaks and
// "await" for what it expects.
type Event struct {
	Type           string          `json:"type"`
	Direction      string          `json:"direction"`
	Reconciliation *Reconciliation `json:"reconciliation,omitempty"`
}

// Reconciliation is the plugin practice: the absence station and the
// gap-fill enrichment a plugin ships for every event type it declares. A
// station that serves no tool declares none, and says so by name rather than
// by omission.
type Reconciliation struct {
	Declared   bool   `json:"declared"`
	Absence    string `json:"absence,omitempty"`
	Enrichment string `json:"enrichment,omitempty"`
}

// Koine is the extension: everything the paradigm adds that the loader does
// not read. Nothing here is a second copy of a fact above the line — these
// are the same facts said in full, where the shorter forms above are what
// the loader asked for.
type Koine struct {
	Stratum    string      `json:"stratum"` // observer | execution
	Lineage    []string    `json:"lineage"` // namespace lineage, deepest first
	Tool       string      `json:"tool,omitempty"`
	Connection *Connection `json:"connection,omitempty"`
	Events     []Event     `json:"events"`
	Awaits     []Await     `json:"awaits"`
	Emits      []Emit      `json:"emits"`
	Exchanges  []Speaks    `json:"exchanges"`
	Seats      []SeatNeed  `json:"seats"`
	PassUp     PassUp      `json:"passUp"`
}

// PassUp is the station's pass-up surface, derived from its body: how it
// hands its parent the event object as the walk happens.
//
// It rides UNDER the koine key and nowhere else. The host is normative about
// the keys it reads, and koinehost.Manifest has no pass-up field yet — the
// paired ticket (conduit-go#210) has not been written. Emitting an
// above-the-line key the loader does not read would be this SDK deciding the
// host's half of a contract, which is the one thing K2 established it must
// not do. When #210 names a key, it moves up; the derivation does not change.
type PassUp struct {
	// Declared says whether this station's body touches the pass-up at
	// all. It is written even when false, so the absence is stated rather
	// than inferred from a missing key — the same posture Reconciliation
	// takes.
	Declared bool `json:"declared"`
	// Verbs are the ones the body speaks, in the vocabulary the author
	// wrote: passUp, await, withhold.
	Verbs []string `json:"verbs,omitempty"`
	// Hooks are the named hooks the station declares: pre, post.
	Hooks []string `json:"hooks,omitempty"`
	// Awaits says the station asks for its parent's conclusion. Asking is
	// declaring: it is how a station says a parent's finding will arrive
	// as a value it handles, rather than unwinding the walk.
	Awaits bool `json:"awaits"`
	// Type and WithholdType are the reserved exchange types this build
	// speaks. They are declared so a host can refuse a spelling mismatch
	// at load instead of misrouting at run time — and because neither
	// string is asserted anywhere in the engine yet, saying them out loud
	// is what makes a disagreement visible.
	Type         string `json:"type,omitempty"`
	WithholdType string `json:"withholdType,omitempty"`
}

// Await is one declared expectation, content-hash pinned so the door can
// bind it to the expression store the registration cites.
type Await struct {
	Type   string `json:"type"`
	Mode   string `json:"mode"`
	Anchor string `json:"anchor,omitempty"`
	Hash   string `json:"hash"`
}

// Emit is one utterance type the body can speak, named in both vocabularies:
// the Go type the author wrote and the event type the record stores.
type Emit struct {
	Go        string `json:"go"`
	Type      string `json:"type"`
	Namespace string `json:"namespace"`
}

// Speaks is one exchange the body speaks, with the consumption pattern read
// from the code and the chain role it becomes.
type Speaks struct {
	Seat        string      `json:"seat"`
	Name        string      `json:"name"`
	Consumption Consumption `json:"consumption"`
	ChainRole   string      `json:"chainRole"`
	Returns     string      `json:"returns"`
	Permissions []string    `json:"permissions,omitempty"`
}

// SeatNeed is one seat the station speaks and what filling it requires. An
// unfilled seat is refused at registration BY NAME (§7.3), and the same fact
// answers the deployment's three-state read vocabulary.
type SeatNeed struct {
	Seat        string   `json:"seat"`
	Tool        string   `json:"tool,omitempty"`
	Connection  string   `json:"connection,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}
