package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/koine/manifest"
	"github.com/sol-duara-inc/koine-go/koine/wire"
)

func stationDir(t *testing.T) string {
	return filepath.Join(repoRoot(t), "cmd", "koinegen", "fixtures", "station")
}

func extractFixtures(t *testing.T) map[string]manifest.Manifest {
	t.Helper()
	found, err := Extract(loadFixtureRegistry(t), stationDir(t))
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// TestManifest_WorkedExampleNamesItsIdentityAwaitEmitsAndInlineExchange is
// the phase's named done-condition, asserted item by item on the design
// document's §10 station.
func TestManifest_WorkedExampleNamesItsIdentityAwaitEmitsAndInlineExchange(t *testing.T) {
	m := extractFixtures(t)["DeploymentSteward"]

	if m.SchemaVersion != manifest.SchemaVersion || m.Kind != "station" {
		t.Errorf("manifest is %s/%s", m.SchemaVersion, m.Kind)
	}
	if m.Identity != (manifest.Identity{Group: "payment-engineering", Author: "mchen", Name: "deployment-steward"}) {
		t.Errorf("identity = %#v", m.Identity)
	}
	// The claim the loader reads is the stratum this station speaks from,
	// in the reverse-domain shape the loader holds a claim to.
	if m.Claim != "dev.cdevents.deployment" {
		t.Errorf("claim = %q", m.Claim)
	}
	if m.Koine.Stratum != "observer" {
		t.Errorf("stratum = %q — the steward embeds ObserverBase", m.Koine.Stratum)
	}
	if got := strings.Join(m.Koine.Lineage, ","); got != "dev.cdevents.deployment" {
		t.Errorf("lineage = %q", got)
	}
	if m.Complete != "all-awaited" {
		t.Errorf("complete = %q", m.Complete)
	}

	if len(m.Koine.Awaits) != 1 {
		t.Fatalf("awaits %d shapes, want exactly one", len(m.Koine.Awaits))
	}
	a := m.Koine.Awaits[0]
	if a.Type != "dev.cdevents.deployment" || a.Mode != "resolved" || a.Anchor != "deploy" {
		t.Errorf("await = %#v", a)
	}
	if !strings.HasPrefix(a.Hash, "sha256:") || len(a.Hash) != len("sha256:")+64 {
		t.Errorf("await is not content-hash pinned: %q", a.Hash)
	}

	emits := map[string]bool{}
	for _, e := range m.Koine.Emits {
		emits[e.Type] = true
	}
	for _, want := range []string{"dev.cdevents.deployment.recorded", "dev.cdevents.deployment.requested"} {
		if !emits[want] {
			t.Errorf("the manifest does not name emit %q", want)
		}
	}
	if len(m.Koine.Events) != len(m.Koine.Emits) {
		t.Errorf("the declared events (%d) and the Koine emits (%d) disagree", len(m.Koine.Events), len(m.Koine.Emits))
	}
	// The short forms the loader reads and the long forms below the line
	// are one fact said twice, from one source: they cannot disagree.
	if len(m.Emits) != len(m.Koine.Emits) || len(m.Awaits) != len(m.Koine.Awaits) ||
		len(m.Exchanges) != len(m.Koine.Exchanges) {
		t.Errorf("the loader's lists and the Koine section disagree: %#v", m)
	}
	if strings.Join(m.Awaits, ",") != "dev.cdevents.deployment" {
		t.Errorf("awaits = %v", m.Awaits)
	}
	if strings.Join(m.Exchanges, ",") != "history.last" {
		t.Errorf("exchanges = %v", m.Exchanges)
	}

	if len(m.Koine.Exchanges) != 1 {
		t.Fatalf("the steward speaks %d exchanges, want one", len(m.Koine.Exchanges))
	}
	ex := m.Koine.Exchanges[0]
	if ex.Name != "history.last" || ex.Seat != "history" {
		t.Errorf("exchange = %#v", ex)
	}
	if ex.Consumption != manifest.Inline {
		t.Errorf("history.last is %q — Value() is consumed, so it is inline", ex.Consumption)
	}
	if ex.ChainRole != "main" {
		t.Errorf("inline must be the caller's own chain, got role %q", ex.ChainRole)
	}
	if len(m.Koine.Seats) != 1 || m.Koine.Seats[0].Seat != "history" || m.Koine.Seats[0].Tool != "record" {
		t.Errorf("seats = %#v", m.Koine.Seats)
	}
}

// TestManifest_ConsumptionTopologyIsTheChainRoles pins A4: the two
// consumption patterns a station's own body can produce ARE two of the
// chain roles the engine's stored expected graph admits. A third role is
// minted from a workflow's own topology declaration and is never produced
// by a station's consumption (DESIGN.md §6, amended 2026-08-27, issue #11)
// — so it is not tested here.
func TestManifest_ConsumptionTopologyIsTheChainRoles(t *testing.T) {
	found := extractFixtures(t)
	roles := map[manifest.Consumption]string{}
	for _, station := range []string{"DeploymentSteward", "DeploymentAuditor"} {
		for _, ex := range found[station].Koine.Exchanges {
			roles[ex.Consumption] = ex.ChainRole
		}
	}
	want := map[manifest.Consumption]string{
		manifest.Inline:     "main",
		manifest.Concurrent: "blocking",
	}
	for how, role := range want {
		got, ok := roles[how]
		if !ok {
			t.Errorf("no fixture station demonstrates %q — the topology claim is untested", how)
			continue
		}
		if got != role {
			t.Errorf("%q maps to chain role %q, want %q", how, got, role)
		}
	}
	if len(roles) != 2 {
		t.Errorf("the fixtures produced %d consumption patterns, want exactly the two roles", len(roles))
	}

	auditor := found["DeploymentAuditor"]
	if auditor.Koine.Stratum != "execution" {
		t.Errorf("the auditor embeds ExecutionBase; stratum = %q", auditor.Koine.Stratum)
	}
	if len(auditor.Koine.Exchanges) != 1 {
		t.Fatalf("the auditor speaks %d exchanges, want one", len(auditor.Koine.Exchanges))
	}
	if auditor.Koine.Exchanges[0].Name != "history.last" || auditor.Koine.Exchanges[0].Consumption != manifest.Concurrent {
		t.Errorf("spoken-and-never-consumed must be concurrent: %#v", auditor.Koine.Exchanges[0])
	}
}

// TestManifest_GoldensAreByteStable keeps the committed manifests honest the
// same way the strata are kept honest: extraction is a function of the code
// and the registry, so re-extracting must produce the same bytes.
func TestManifest_GoldensAreByteStable(t *testing.T) {
	found := extractFixtures(t)
	goldens := map[string]string{
		"DeploymentSteward":   "deployment-steward.json",
		"DeploymentAuditor":   "deployment-auditor.json",
		"DeploymentRehearsal": "deployment-rehearsal.json",
	}
	dir := filepath.Join(repoRoot(t), "cmd", "koinegen", "fixtures", "manifests")
	for station, file := range goldens {
		m, ok := found[station]
		if !ok {
			t.Fatalf("the fixtures declare no station named %s", station)
		}
		data, err := m.JSON()
		if err != nil {
			t.Fatal(err)
		}
		committed, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Errorf("%s is not committed: %v", file, err)
			continue
		}
		if string(committed) != string(data) {
			t.Errorf("%s drifted from its golden — run `go generate ./cmd/koinegen/fixtures/`", file)
		}
	}
}

// TestManifest_HandWrittenManifestsAreRefusedWork pins A3's harder half. It
// is not enough that koinegen derives the manifest; a hand-written one must
// be refused, by name, rather than quietly preferred or quietly merged.
func TestManifest_HandWrittenManifestsAreRefusedWork(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"doc.go", "steward.go", "auditor.go", "rehearsal.go"} {
		raw, err := os.ReadFile(filepath.Join(stationDir(t), name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Derivation works from the copied source alone.
	if _, err := Extract(loadFixtureRegistry(t), dir); err != nil {
		t.Fatalf("the copied station could not be derived: %v", err)
	}
	// Now put a hand-written declaration beside it.
	handWritten := `{"schemaVersion":"koine.manifest/1","claim":{"group":"someone-else","author":"nobody","name":"not-this-station"}}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(handWritten), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := Extract(loadFixtureRegistry(t), dir)
	if err == nil {
		t.Fatal("a hand-written manifest was tolerated — a declaration a human typed can lie about the body")
	}
	if found != nil {
		t.Fatal("a refusal stored manifests — nothing is stored on refusal")
	}
	if !strings.Contains(err.Error(), "manifest.json") || !strings.Contains(err.Error(), "derived from code") {
		t.Fatalf("the refusal did not name the file and the law: %v", err)
	}
}

// TestManifest_AwaitsArePinnedByContent pins §7's content hash: the same
// declaration hashes the same, and a changed anchor or a changed mode is a
// different pin. A selector cannot be edited after registration without the
// manifest saying so.
func TestManifest_AwaitsArePinnedByContent(t *testing.T) {
	base := manifest.Await{Type: "dev.cdevents.deployment", Mode: "resolved", Anchor: "deploy"}
	pin := awaitHash(base)
	if awaitHash(base) != pin {
		t.Fatal("the same selector pinned differently twice")
	}
	moved := base
	moved.Anchor = "rollout"
	if awaitHash(moved) == pin {
		t.Error("a moved anchor kept its pin")
	}
	filtered := base
	filtered.Mode = "event"
	if awaitHash(filtered) == pin {
		t.Error("a changed mode kept its pin")
	}
	if got := extractFixtures(t)["DeploymentSteward"].Koine.Awaits[0].Hash; got != pin {
		t.Errorf("the steward's await pinned to %q, want %q", got, pin)
	}
}

// TestManifest_RefusesAStationItCannotDerive pins the refusal ladder on the
// extraction side: half a station is refused whole, by name.
func TestManifest_RefusesAStationItCannotDerive(t *testing.T) {
	cases := []struct {
		name, body, wantIn string
	}{
		{
			name: "a base with no Resolve",
			body: `package half

import "github.com/sol-duara-inc/koine-go/koine"

type Halfling struct{ koine.ObserverBase }

func (Halfling) Identity() koine.Identity { return koine.Identity{Group: "g", Author: "a", Name: "n"} }
`,
			wantIn: "declares no Awaits",
		},
		{
			name: "an identity that is not written down",
			body: `package half

import (
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
)

var claim = koine.Identity{Group: "g", Author: "a", Name: "n"}

type Halfling struct{ koine.ObserverBase }

func (Halfling) Identity() koine.Identity            { return claim }
func (Halfling) Awaits() []selector.Selector         { return selector.List(deployment.Resolved()) }
func (Halfling) Complete() koine.Contract            { return koine.DefaultAllAwaited }
func (Halfling) Resolve(d koine.Delivery, y koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
	y(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}
`,
			wantIn: "returns no koine.Identity literal",
		},
		{
			name: "a body that never speaks",
			body: `package half

import (
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
)

type Halfling struct{ koine.ObserverBase }

func (Halfling) Identity() koine.Identity    { return koine.Identity{Group: "g", Author: "a", Name: "n"} }
func (Halfling) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (Halfling) Complete() koine.Contract    { return koine.DefaultAllAwaited }
func (Halfling) Resolve(d koine.Delivery, y koine.Yield) {
	_ = d.(deployment.ResolvedDelivery)
}
`,
			wantIn: "yields nothing",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "half.go"), []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			found, err := Extract(loadFixtureRegistry(t), dir)
			if err == nil {
				t.Fatalf("derived a manifest from %s", c.name)
			}
			if found != nil {
				t.Fatal("a refusal stored manifests")
			}
			if !strings.Contains(err.Error(), c.wantIn) {
				t.Fatalf("the refusal did not say %q: %v", c.wantIn, err)
			}
		})
	}
}

// extractStation writes one station source into a directory of its own and
// derives it. The bodies below are valid Go; they are never compiled here
// because the extractor is a reader of source, and what is under test is
// exactly what it reads.
func extractStation(t *testing.T, body string) (map[string]manifest.Manifest, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return Extract(loadFixtureRegistry(t), dir)
}

const probeHead = `package probe

import (
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
)

type Probe struct{ koine.ObserverBase }

func (Probe) Identity() koine.Identity    { return koine.Identity{Group: "g", Author: "a", Name: "n"} }
func (Probe) Awaits() []selector.Selector { return selector.List(deployment.Resolved()) }
func (Probe) Complete() koine.Contract    { return koine.DefaultAllAwaited }

func (p Probe) Resolve(d koine.Delivery, yield koine.Yield) {
	dep := d.(deployment.ResolvedDelivery)
`

const probeTail = `
	yield(deployment.DeploymentRecorded{Artifact: dep.ArtifactID})
}

func (p Probe) helper(dep deployment.ResolvedDelivery, yield koine.Yield) {
	dep.History().Last(koine.Success)
}

func (p Probe) speakFor(dep deployment.ResolvedDelivery) {
	dep.History().Last(koine.Success)
}

func (p Probe) gate(h koine.Handle[deployment.DeploymentFinished]) {}

func (p Probe) atSeat(s deployment.HistorySeat) {}
`

// TestManifest_ConsumptionFollowsTheHandleThroughItsBinding pins the reading
// the analyzer must get right: the pattern is a property of what the body
// DOES with the handle, never of how the call happened to be spelled.
// Binding is not a style — it is the only way to reach Received().
func TestManifest_ConsumptionFollowsTheHandleThroughItsBinding(t *testing.T) {
	cases := []struct {
		name, body string
		want       manifest.Consumption
		role       string
	}{
		{
			name: "chained and consumed",
			body: "\t_, _ = dep.History().Last(koine.Success).Value()",
			want: manifest.Inline, role: "main",
		},
		{
			name: "bound, then consumed",
			body: "\th := dep.History().Last(koine.Success)\n\t_, _ = h.Value()",
			want: manifest.Inline, role: "main",
		},
		{
			name: "bound, gated on the fast beat, then consumed",
			body: "\th := dep.History().Last(koine.Success)\n\t_ = h.Received()\n\t_, _ = h.Value()",
			want: manifest.Inline, role: "main",
		},
		{
			name: "bound and gated on the fast beat only",
			body: "\th := dep.History().Last(koine.Success)\n\t_ = h.Received()",
			want: manifest.Concurrent, role: "blocking",
		},
		{
			name: "spoken and walked away from",
			body: "\tdep.History().Last(koine.Success)",
			want: manifest.Concurrent, role: "blocking",
		},
		{
			name: "bound to the blank identifier",
			body: "\t_ = dep.History().Last(koine.Success)",
			want: manifest.Concurrent, role: "blocking",
		},
		{
			name: "spoken through a bound seat",
			body: "\tseat := dep.History()\n\t_, _ = seat.Last(koine.Success).Value()",
			want: manifest.Inline, role: "main",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			found, err := extractStation(t, probeHead+c.body+probeTail)
			if err != nil {
				t.Fatalf("the analyzer refused a body it can read: %v", err)
			}
			ex := found["Probe"].Koine.Exchanges
			if len(ex) != 1 {
				t.Fatalf("the probe speaks %d exchanges, want one: %#v", len(ex), ex)
			}
			if ex[0].Name != "history.last" {
				t.Fatalf("the probe spoke %q", ex[0].Name)
			}
			if ex[0].Consumption != c.want || ex[0].ChainRole != c.role {
				t.Errorf("read as %q/%q, want %q/%q", ex[0].Consumption, ex[0].ChainRole, c.want, c.role)
			}
			if len(found["Probe"].Koine.Seats) != 1 || found["Probe"].Koine.Seats[0].Seat != "history" {
				t.Errorf("the seat did not reach the manifest: %#v", found["Probe"].Koine.Seats)
			}
		})
	}
}

// TestManifest_RefusesWhatItCannotReadWithCertainty is A9 applied where it
// binds hardest. The manifest is a contract the engine mints coordinates and
// budget from, so a reading the analyzer had to guess at is worse than no
// manifest: it is a declaration that lies about the code, which is the exact
// thing A3 exists to prevent. Every body below is ordinary Go that this
// analyzer cannot read with certainty, and every one of them is refused by
// name rather than defaulted.
func TestManifest_RefusesWhatItCannotReadWithCertainty(t *testing.T) {
	cases := []struct {
		name, body, wantIn string
	}{
		{
			name: "a handle name reused for a second handle",
			body: "\th := dep.Ledger().Note(\"consumed, on purpose\")\n" +
				"\t_, _ = h.Value()\n" +
				"\th = dep.Ledger().Note(\"gated, on purpose\")\n\t_ = h",
			wantIn: `binds "h" 2 times`,
		},
		{
			name: "a handle shadowed in an inner scope",
			body: "\th := dep.History().Last(koine.Success)\n" +
				"\t_, _ = h.Value()\n" +
				"\tif dep.Outcome == koine.Failure {\n\t\th := dep.Ledger().Note(\"inner\")\n\t\t_ = h\n\t}",
			wantIn: `binds "h" 2 times`,
		},
		{
			name:   "a handle handed to a helper",
			body:   "\th := dep.History().Last(koine.Success)\n\tp.gate(h)",
			wantIn: "hands the handle",
		},
		{
			name:   "a handle handed straight to a helper",
			body:   "\tp.gate(dep.History().Last(koine.Success))",
			wantIn: "hands the handle to",
		},
		{
			name:   "a seat handed to a helper",
			body:   "\tp.atSeat(dep.History())",
			wantIn: `hands the "history" seat`,
		},
		{
			name:   "a seat bound and then handed to a helper",
			body:   "\tseat := dep.History()\n\tp.atSeat(seat)",
			wantIn: `hands the "history" seat`,
		},
		{
			name:   "the delivery handed to a helper that speaks",
			body:   "\tp.speakFor(dep)",
			wantIn: "hands its delivery",
		},
		{
			name:   "the yield handed away",
			body:   "\tp.helper(deployment.ResolvedDelivery{}, yield)",
			wantIn: "hands its koine.Yield argument",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			found, err := extractStation(t, probeHead+c.body+probeTail)
			if err == nil {
				t.Fatalf("the analyzer read a body it cannot read with certainty, and declared: %#v", found["Probe"].Koine)
			}
			if found != nil {
				t.Fatal("a refusal stored manifests — nothing is stored on refusal")
			}
			if !strings.Contains(err.Error(), c.wantIn) {
				t.Fatalf("the refusal did not say %q: %v", c.wantIn, err)
			}
			if !strings.Contains(err.Error(), "Probe.Resolve") {
				t.Fatalf("the refusal did not name the station: %v", err)
			}
		})
	}
}

// TestManifest_ThePassUpSurfaceIsDerivedFromTheBody is koine-go#5's sixth
// done-condition: the manifest declares what the station's pass-up surface
// uses, derived the same way everything else here is — from what the author
// wrote, never from a declaration beside it.
func TestManifest_ThePassUpSurfaceIsDerivedFromTheBody(t *testing.T) {
	found := extractFixtures(t)

	cases := map[string]struct {
		verbs, hooks string
		awaits       bool
		declared     bool
	}{
		// The twins: one writes the verbs, one declares the hooks, and
		// the manifest says which — because they are the same behaviour
		// spelled two ways, and a declaration that hid the difference
		// would be a declaration saying less than it knows.
		"ChainVerbs": {verbs: "passUp,await", awaits: true, declared: true},
		"ChainHooks": {hooks: "pre,post", awaits: true, declared: true},
		// All three verbs, across two branches.
		"ChainWalker": {verbs: "passUp,await,withhold", awaits: true, declared: true},
		// Zero code: the step-end default pass is the host's duty, and a
		// station that wrote nothing declares nothing.
		"DeploymentSteward":   {},
		"DeploymentAuditor":   {},
		"DeploymentRehearsal": {},
	}
	for station, want := range cases {
		t.Run(station, func(t *testing.T) {
			got := found[station].Koine.PassUp
			if got.Declared != want.declared {
				t.Errorf("declared = %v, want %v", got.Declared, want.declared)
			}
			if strings.Join(got.Verbs, ",") != want.verbs {
				t.Errorf("verbs = %v, want %q", got.Verbs, want.verbs)
			}
			if strings.Join(got.Hooks, ",") != want.hooks {
				t.Errorf("hooks = %v, want %q", got.Hooks, want.hooks)
			}
			if got.Awaits != want.awaits {
				t.Errorf("awaits = %v, want %v", got.Awaits, want.awaits)
			}
			// A station that declares nothing pins no wire spelling:
			// the strings are only the business of a station that
			// actually speaks them.
			if want.declared && (got.Type == "" || got.WithholdType == "") {
				t.Errorf("a declaring station must name the spellings it speaks: %#v", got)
			}
			if !want.declared && (got.Type != "" || got.WithholdType != "") {
				t.Errorf("a silent station named a spelling: %#v", got)
			}
		})
	}
}

// TestManifest_TheReservedSpellingsAreWrittenDownTwiceAndPinnedOnce keeps the
// analyzer and the guest contract from drifting. cmd/koinegen cannot import
// koine/wire — a build tool has no business depending on the guest contract —
// so the two strings live in both places, and this is the pin that makes a
// divergence a failing test.
func TestManifest_TheReservedSpellingsAreWrittenDownTwiceAndPinnedOnce(t *testing.T) {
	if wirePassUpType != wire.TypePassUp {
		t.Errorf("koinegen says %q, koine/wire says %q", wirePassUpType, wire.TypePassUp)
	}
	if wireWithholdType != wire.TypeWithhold {
		t.Errorf("koinegen says %q, koine/wire says %q", wireWithholdType, wire.TypeWithhold)
	}
}

// TestManifest_PostWithoutPreIsRefusedByName pins the A9 posture on the hook
// pair. A station that declared Post and not Pre asked what its parent
// concluded about an object it never minted; koinegen refuses it rather than
// letting it discover the gap in the sandbox.
func TestManifest_PostWithoutPreIsRefusedByName(t *testing.T) {
	body := probeHead + "\t_ = dep.ArtifactID\n" + probeTail + `
func (p Probe) Post(u koine.Utterance, c koine.Conclusion) {}
`
	found, err := extractStation(t, body)
	if err == nil {
		t.Fatalf("a station declaring Post without Pre was derived: %#v", found["Probe"].Koine.PassUp)
	}
	if found != nil {
		t.Fatal("a refusal stored manifests")
	}
	if !strings.Contains(err.Error(), "Post without Pre") {
		t.Fatalf("the refusal did not name the pair: %v", err)
	}
}
