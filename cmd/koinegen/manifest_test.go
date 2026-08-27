package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/koine/manifest"
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
	if m.Claim != (manifest.Claim{Group: "payment-engineering", Author: "mchen", Name: "deployment-steward"}) {
		t.Errorf("claim = %#v", m.Claim)
	}
	if m.Koine.Stratum != "observer" {
		t.Errorf("stratum = %q — the steward embeds ObserverBase", m.Koine.Stratum)
	}
	if got := strings.Join(m.Koine.Lineage, ","); got != "dev.cdevents.deployment" {
		t.Errorf("lineage = %q", got)
	}
	if m.Koine.Complete != "all-awaited" {
		t.Errorf("complete = %q", m.Koine.Complete)
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
	if len(m.Events) != len(m.Koine.Emits) {
		t.Errorf("the engine family's events (%d) and the Koine emits (%d) disagree", len(m.Events), len(m.Koine.Emits))
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

// TestManifest_ConsumptionTopologyIsTheChainRoles pins A4: the three
// consumption patterns ARE the three chain roles the engine's stored
// expected graph admits, and nothing else is admitted.
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
		manifest.Detached:   "detached",
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
	if len(roles) != 3 {
		t.Errorf("the fixtures produced %d consumption patterns, want exactly the three roles", len(roles))
	}

	auditor := found["DeploymentAuditor"]
	if auditor.Koine.Stratum != "execution" {
		t.Errorf("the auditor embeds ExecutionBase; stratum = %q", auditor.Koine.Stratum)
	}
	if len(auditor.Koine.Exchanges) != 2 {
		t.Fatalf("the auditor speaks %d exchanges, want two", len(auditor.Koine.Exchanges))
	}
	if auditor.Koine.Exchanges[0].Name != "history.last" || auditor.Koine.Exchanges[0].Consumption != manifest.Concurrent {
		t.Errorf("spoken-and-never-consumed must be concurrent: %#v", auditor.Koine.Exchanges[0])
	}
	if auditor.Koine.Exchanges[1].Name != "ledger.note" || auditor.Koine.Exchanges[1].Consumption != manifest.Detached {
		t.Errorf("koine.Detach must be detached: %#v", auditor.Koine.Exchanges[1])
	}
}

// TestManifest_GoldensAreByteStable keeps the committed manifests honest the
// same way the strata are kept honest: extraction is a function of the code
// and the registry, so re-extracting must produce the same bytes.
func TestManifest_GoldensAreByteStable(t *testing.T) {
	found := extractFixtures(t)
	goldens := map[string]string{
		"DeploymentSteward": "deployment-steward.json",
		"DeploymentAuditor": "deployment-auditor.json",
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
	for _, name := range []string{"doc.go", "steward.go", "auditor.go"} {
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
