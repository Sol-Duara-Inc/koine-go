package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// repoRoot, registryDir and fixturesDir locate the committed inputs and
// outputs from wherever the test binary runs.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller info")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(self))) // cmd/koinegen -> cmd -> root
}

func registryDir(t *testing.T) string {
	return filepath.Join(repoRoot(t), "cmd", "koinegen", "testdata", "registry")
}

func strataDir(t *testing.T) string {
	return filepath.Join(repoRoot(t), "cmd", "koinegen", "fixtures", "strata")
}

const fixturePkgBase = "github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata"

func loadFixtureRegistry(t *testing.T) *Registry {
	t.Helper()
	reg, err := LoadRegistry(registryDir(t))
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// TestGenerate_GoldensAreByteStable is the phase's hard gate: a full
// regeneration produces zero diff. Generation is a function of the registry
// alone — no clock, no map order, no host state — so the committed tree and
// a fresh render are the same bytes or the generator is not deterministic.
func TestGenerate_GoldensAreByteStable(t *testing.T) {
	reg := loadFixtureRegistry(t)
	files, err := GenerateFiles(reg, fixturePkgBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("the registry generated nothing")
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		committed, err := os.ReadFile(filepath.Join(strataDir(t), filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("%s is not committed: %v", path, err)
			continue
		}
		if string(committed) != string(files[path]) {
			t.Errorf("%s drifted from its golden — run `go generate ./cmd/koinegen/fixtures/`", path)
		}
	}

	// Nothing committed that generation no longer produces.
	found, err := filepath.Glob(filepath.Join(strataDir(t), "*", "*.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, abs := range found {
		rel, err := filepath.Rel(strataDir(t), abs)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := files[filepath.ToSlash(rel)]; !ok {
			t.Errorf("%s is committed but nothing generates it any more", rel)
		}
	}

	// And rendering twice from the same registry is the same bytes: the
	// determinism claim, made against the generator rather than the disk.
	again, err := GenerateFiles(reg, fixturePkgBase)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if string(again[path]) != string(files[path]) {
			t.Errorf("%s renders differently on a second pass", path)
		}
	}
}

// TestGenerate_TheFloorDoesNotKnowItsExtensions pins downward blindness as a
// property of the OUTPUT, not a promise: a floor package's bytes are
// identical whether or not a vendor and a customer extend it. Confidentiality
// that cannot be broken costs nothing to keep.
func TestGenerate_TheFloorDoesNotKnowItsExtensions(t *testing.T) {
	full := loadFixtureRegistry(t)
	withExtensions, err := GenerateFiles(full, fixturePkgBase)
	if err != nil {
		t.Fatal(err)
	}

	// The same floor namespace alone in its own registry.
	alone := t.TempDir()
	src := filepath.Join(registryDir(t), "dev.cdevents.build.json")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alone, "dev.cdevents.build.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	solo, err := LoadRegistry(alone)
	if err != nil {
		t.Fatal(err)
	}
	withoutExtensions, err := GenerateFiles(solo, fixturePkgBase)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"build/stratum.gen.go", "build/delivery.gen.go"} {
		if string(withExtensions[path]) != string(withoutExtensions[path]) {
			t.Errorf("%s changed because something extends it — the floor learned about a vendor", path)
		}
	}
}

// TestGenerate_SeedingConstructorsLiveAtTheOwningStratumOnly reads the
// committed source: each stratum's file carries its own seeder and nobody
// else's. A floor package that seeded a vendor's object would be a floor
// package that knows a vendor exists.
func TestGenerate_SeedingConstructorsLiveAtTheOwningStratumOnly(t *testing.T) {
	seeders := map[string]string{
		"build/stratum.gen.go":      "func SeedBuildFinished(",
		"jenkins/stratum.gen.go":    "func SeedJenkinsBuildFinished(",
		"payments/stratum.gen.go":   "func SeedPaymentsBuildFinished(",
		"deployment/stratum.gen.go": "func SeedDeploymentFinished(",
	}
	sources := map[string]string{}
	for path := range seeders {
		raw, err := os.ReadFile(filepath.Join(strataDir(t), filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		sources[path] = string(raw)
	}
	for path, own := range seeders {
		if !strings.Contains(sources[path], own) {
			t.Errorf("%s does not declare %s", path, own)
		}
		for other, foreign := range seeders {
			if other == path {
				continue
			}
			if strings.Contains(sources[path], foreign) {
				t.Errorf("%s declares %s, which belongs to %s", path, foreign, other)
			}
		}
	}
}

// TestGenerate_NothingGeneratedReflects pins A6 at the level that matters:
// the guest target stays open because no generated line reaches for
// reflection or for the reflecting marshaller.
func TestGenerate_NothingGeneratedReflects(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(strataDir(t), "*", "*.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no generated files to inspect")
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{`"reflect"`, `"encoding/json"`} {
			if strings.Contains(string(raw), forbidden) {
				t.Errorf("%s imports %s — generated code is reflection-free by law", filepath.Base(path), forbidden)
			}
		}
	}
}

// TestGenerate_ProjectionRefusesAVendorKeyAtTheFloor is the compile-refusal
// fixture. Projection is the TYPE SYSTEM: a floor station reaching for a
// vendor key does not compile, because the key does not exist from that
// position. The control station one stratum down must compile — without the
// passing control the failure would prove nothing.
func TestGenerate_ProjectionRefusesAVendorKeyAtTheFloor(t *testing.T) {
	root := repoRoot(t)

	build := func(t *testing.T, body string) (string, error) {
		t.Helper()
		dir := t.TempDir()
		gomod := "module projectioncheck\n\ngo 1.23\n\nrequire github.com/sol-duara-inc/koine-go v0.0.0\n\nreplace github.com/sol-duara-inc/koine-go => " + root + "\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	const violation = `package main

import (
	floor "github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/build"
	"github.com/sol-duara-inc/koine-go/koine"
)

type floorWatcher struct{ koine.ObserverBase }

func (floorWatcher) Resolve(d koine.Delivery, yield koine.Yield) {
	b := d.(floor.FinishedDelivery)
	_ = b.ExecutorNode // a vendor key, reached for from the community floor
}

func main() {}
`
	out, err := build(t, violation)
	if err == nil {
		t.Fatal("a floor station read a vendor key and compiled — the projection wall is down")
	}
	if !strings.Contains(out, "ExecutorNode") {
		t.Fatalf("the build failed for the wrong reason:\n%s", out)
	}

	const control = `package main

import (
	vendor "github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/jenkins"
	"github.com/sol-duara-inc/koine-go/koine"
)

type vendorWatcher struct{ koine.ObserverBase }

func (vendorWatcher) Resolve(d koine.Delivery, yield koine.Yield) {
	b := d.(vendor.FinishedDelivery)
	_ = b.ExecutorNode // the same key, from the stratum that owns it
	_ = b.Subject      // and the floor's, embedded downward
}

func main() {}
`
	if out, err := build(t, control); err != nil {
		t.Fatalf("the vendor-stratum control failed to compile — the refusal above proves nothing:\n%s", out)
	}
}

// TestGenerate_RegistryRefusesByName pins A9's posture on the way in: the
// registry is judged in order and refused by name, and a refusal stores
// nothing — no Registry comes back, so no half-generated tree can exist.
func TestGenerate_RegistryRefusesByName(t *testing.T) {
	cases := []struct {
		name, file, body, wantIn string
	}{
		{
			name: "a stratum above the floor must extend something",
			file: "io.orphan.json",
			body: `{"namespace":"io.orphan","package":"orphan","stratum":"vendor","doc":"x"}`,
		},
		{
			name:   "a type cannot shadow an inherited key",
			file:   "io.shadow.json",
			body:   `{"namespace":"io.shadow","package":"shadow","stratum":"vendor","extends":"dev.cdevents.build","doc":"x","types":[{"name":"ShadowBuildFinished","event":"e","extends":"BuildFinished","doc":"x","fields":[{"name":"Subject","json":"subject2","type":"string"}]}]}`,
			wantIn: "Subject",
		},
		{
			name:   "a delivery cannot collide with a type name",
			file:   "io.collideident.json",
			body:   `{"namespace":"io.collideident","package":"collideident","stratum":"floor","doc":"x","refs":[{"name":"R","doc":"x"}],"types":[{"name":"Deploy","event":"e","doc":"x","fields":[{"name":"F","json":"f","type":"string"}]}],"deliveries":[{"name":"DeployDelivery","of":"Deploy","doc":"x","await":{"mode":"event","type":"e"}}]}`,
			wantIn: `would declare "Deploy" twice`,
		},
		{
			name:   "two seats in one namespace cannot share a verb name",
			file:   "io.collideseat.json",
			body:   `{"namespace":"io.collideseat","package":"collideseat","stratum":"floor","doc":"x","types":[{"name":"T","event":"e","doc":"x","fields":[{"name":"F","json":"f","type":"string"}]}],"deliveries":[{"name":"ADelivery","of":"T","doc":"x","await":{"mode":"event","type":"e"},"verbs":[{"name":"History","seat":"h","doc":"x","intents":[{"name":"Ask","exchange":"h.ask","returns":"T","doc":"x"}]}]},{"name":"BDelivery","of":"T","doc":"x","await":{"mode":"event","type":"e"},"verbs":[{"name":"History","seat":"h2","doc":"x","intents":[{"name":"Ask","exchange":"h2.ask","returns":"T","doc":"x"}]}]}]}`,
			wantIn: `would declare "HistorySeat" twice`,
		},
		{
			name:   "a field type outside the grammar",
			file:   "io.grammar.json",
			body:   `{"namespace":"io.grammar","package":"grammar","stratum":"floor","doc":"x","types":[{"name":"T","event":"e","doc":"x","fields":[{"name":"F","json":"f","type":"float64"}]}]}`,
			wantIn: "field type must be one of",
		},
		{
			name:   "an intent that answers with an unknown type",
			file:   "io.unknown.json",
			body:   `{"namespace":"io.unknown","package":"unknown","stratum":"floor","doc":"x","types":[{"name":"T","event":"e","doc":"x","fields":[{"name":"F","json":"f","type":"string"}]}],"deliveries":[{"name":"TDelivery","of":"T","doc":"x","await":{"mode":"event","type":"e"},"verbs":[{"name":"Seat","seat":"s","doc":"x","intents":[{"name":"Ask","exchange":"s.ask","returns":"Nope","doc":"x"}]}]}]}`,
			wantIn: "Nope",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, keep := range []string{"dev.cdevents.build.json"} {
				raw, err := os.ReadFile(filepath.Join(registryDir(t), keep))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, keep), raw, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, c.file), []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			reg, err := LoadRegistry(dir)
			if err == nil {
				t.Fatal("the registry was admitted")
			}
			if reg != nil {
				t.Fatal("a refusal stored a registry — nothing is stored on refusal")
			}
			if c.wantIn != "" && !strings.Contains(err.Error(), c.wantIn) {
				t.Fatalf("the refusal did not name %q: %v", c.wantIn, err)
			}
		})
	}
}

// TestGenerate_RefusalDoesNotDependOnNamespaceOrder pins that the registry's
// guards reach the whole lineage regardless of where a namespace sorts.
// Namespaces are loaded and judged in name order; linking a type to the type
// it extends therefore has to be a pass of its own, run for every namespace
// BEFORE any of them is judged. Without that, a namespace sorting ahead of
// its ancestor walks a one-link chain, the floor's fields are invisible, and
// the guard's reach is decided by the alphabet.
//
// The two schemas below are the same schema under two names — one sorting
// before its ancestor, one after. Both must be refused, and for the same
// reason.
func TestGenerate_RefusalDoesNotDependOnNamespaceOrder(t *testing.T) {
	collisions := []struct {
		name, kind, wantIn string
	}{
		{
			name:   "a Go field shadowing an inherited one",
			kind:   `{"name":"Subject","json":"subjectAgain","type":"string"}`,
			wantIn: "shadows inherited field",
		},
		{
			// The silent one: a distinct Go name reusing the floor's wire
			// key compiles, and round-trips wrong — the same key written
			// twice, the inherited value lost on the way back.
			name:   "a distinct Go field reusing an inherited wire key",
			kind:   `{"name":"Region","json":"subject","type":"string"}`,
			wantIn: `reuses wire key "subject"`,
		},
	}
	// The collision is with a FLOOR key, two links up, so the chain the
	// guard has to walk is longer than one. "com.…" sorts before both of
	// its ancestors; "org.…" sorts after both.
	orderings := map[string]string{
		"sorting before its ancestors": "com.example.collide",
		"sorting after its ancestors":  "org.example.collide",
	}

	for _, c := range collisions {
		t.Run(c.name, func(t *testing.T) {
			for ordering, namespace := range orderings {
				dir := t.TempDir()
				for _, ancestor := range []string{"dev.cdevents.build.json", "io.jenkins.json"} {
					raw, err := os.ReadFile(filepath.Join(registryDir(t), ancestor))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, ancestor), raw, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				body := `{"namespace":"` + namespace + `","package":"collide","stratum":"customer",` +
					`"extends":"io.jenkins","doc":"x","types":[{"name":"CollideBuildFinished",` +
					`"event":"dev.cdevents.build.finished","extends":"JenkinsBuildFinished","doc":"x","fields":[` + c.kind + `]}]}`
				if err := os.WriteFile(filepath.Join(dir, namespace+".json"), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
				reg, err := LoadRegistry(dir)
				if err == nil {
					t.Errorf("%s: admitted — the guard's reach is decided by the alphabet", ordering)
					continue
				}
				if reg != nil {
					t.Errorf("%s: a refusal stored a registry", ordering)
				}
				if !strings.Contains(err.Error(), c.wantIn) {
					t.Errorf("%s: the refusal did not say %q: %v", ordering, c.wantIn, err)
				}
			}
		})
	}
}

// TestGenerate_TheCommittedCustomerStratumIsJudgedWhereItSorts is the same
// point aimed at the shipped fixtures: com.example.payments-engineering
// sorts before io.jenkins, so it sat in exactly the blind spot above. It is
// admitted because it collides with nothing — and the collision it does not
// have is now actually looked for.
func TestGenerate_TheCommittedCustomerStratumIsJudgedWhereItSorts(t *testing.T) {
	reg := loadFixtureRegistry(t)
	customer := reg.Namespace("com.example.payments-engineering")
	if customer == nil {
		t.Fatal("the customer stratum is not in the registry")
	}
	want := []string{"com.example.payments-engineering", "io.jenkins", "dev.cdevents.build"}
	if got := customer.Lineage(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("lineage = %v, want %v — a stratum that sorts before its ancestor still sees the whole chain", got, want)
	}
	t.Run("and a collision in it is refused", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"dev.cdevents.build.json", "io.jenkins.json"} {
			raw, err := os.ReadFile(filepath.Join(registryDir(t), name))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		// The reused key belongs to the FLOOR, two links above — the reach
		// that ordering used to decide.
		body := `{"namespace":"com.example.payments-engineering","package":"payments","stratum":"customer",` +
			`"extends":"io.jenkins","doc":"x","types":[{"name":"PaymentsBuildFinished",` +
			`"event":"dev.cdevents.build.finished","extends":"JenkinsBuildFinished","doc":"x","fields":[` +
			`{"name":"Region","json":"subject","type":"string"}]}]}`
		if err := os.WriteFile(filepath.Join(dir, "com.example.payments-engineering.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadRegistry(dir); err == nil {
			t.Fatal("the customer stratum reused the floor's wire key and was admitted")
		} else if !strings.Contains(err.Error(), `reuses wire key "subject"`) {
			t.Fatalf("the refusal did not name the key: %v", err)
		}
	})
}
