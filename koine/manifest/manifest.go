// Package manifest is the shape of §7's registration manifest: what a
// station declares to the door that admits it.
//
// The manifest is DERIVED FROM CODE, never hand-written (A3) — koinegen
// extracts it, and a conformance test pins the extractor's output to what
// the station's body actually speaks. This package therefore carries no
// parser for author-supplied manifests and no builder an author could reach:
// it is the vocabulary and the writer, nothing else.
//
// The shape is the engine's manifest shape family (A2) — claim, tool,
// connection shape, and declared events each with its reconciliation entry —
// extended with the Koine fields: stratum, lineage, awaits, emits, and the
// consumption topology. One loader serves both kinds of guest, so nothing
// here invents a second door.
//
// The concrete field spellings of the engine's half of the family are pinned
// against the real loader in K2/K4, where both sides of the wire contract are
// gated. Until that gate exists this package is the SDK's statement of the
// shape, and it is versioned so a mismatch is a named refusal rather than a
// silent drift.
package manifest

// SchemaVersion names the manifest shape this package writes.
const SchemaVersion = "koine.manifest/1"

// Consumption is how a station consumed a handle it spoke — which IS the
// chain role the exchange takes in the engine's stored expected graph (A4).
type Consumption string

const (
	// Inline: Value() was consumed; the exchange runs in the caller's own
	// sequence, on the same chain.
	Inline Consumption = "inline"
	// Concurrent: the handle was spoken and never consumed; the station's
	// completion gates on the exchange's resolution. The default —
	// nothing is silently ungated.
	Concurrent Consumption = "concurrent"
	// Detached: koine.Detach was spoken over the handle, in code, under
	// the author's name; the exchange is released from the gate.
	Detached Consumption = "detached"
)

// ChainRole is the engine-side name of the same fact: the only three roles
// its stored expected graph admits.
func (c Consumption) ChainRole() string {
	switch c {
	case Inline:
		return "main"
	case Concurrent:
		return "blocking"
	case Detached:
		return "detached"
	}
	return ""
}

// Manifest is one station's registration, derived from its code.
type Manifest struct {
	SchemaVersion string      `json:"schemaVersion"`
	Kind          string      `json:"kind"` // "station"; "plugin" when a tool is served
	Claim         Claim       `json:"claim"`
	Tool          string      `json:"tool,omitempty"`
	Connection    *Connection `json:"connection,omitempty"`
	Events        []Event     `json:"events"`
	Koine         Koine       `json:"koine"`
}

// Claim is the identity the station carries. The SDK carries the claim and
// never verifies it (A8); the door verifies it against what login already
// established.
type Claim struct {
	Group  string `json:"group"`
	Author string `json:"author"`
	Name   string `json:"name"`
}

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

// Koine is the extension: everything the paradigm adds to the family.
type Koine struct {
	Stratum   string     `json:"stratum"` // observer | execution
	Lineage   []string   `json:"lineage"` // namespace lineage, deepest first
	Complete  string     `json:"complete"`
	Awaits    []Await    `json:"awaits"`
	Emits     []Emit     `json:"emits"`
	Exchanges []Speaks   `json:"exchanges"`
	Seats     []SeatNeed `json:"seats"`
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
