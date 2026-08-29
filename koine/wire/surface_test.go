package wire_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/koine/wire"
)

// These tests read the real thing: they build the fixture guests with the
// toolchain the engine builds them with, and then read the wasm module's own
// import and export sections. Nothing here asks Go what it thinks the
// surface is — the surface is whatever the binary declares, and the binary
// is what the engine's loader will hold.

func repoRoot(t *testing.T) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller info")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(self))) // koine/wire -> koine -> root
}

// buildGuest compiles one fixture guest exactly as the engine does. TinyGo
// is the supported guest toolchain (E-A as amended); where it is absent the
// test says what it could not check rather than passing quietly.
func buildGuest(t *testing.T, pkg string) []byte {
	t.Helper()
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("tinygo is not installed: the guest surface cannot be read, and this test proves nothing without it")
	}
	out := filepath.Join(t.TempDir(), filepath.Base(pkg)+".wasm")
	cmd := exec.Command("tinygo", "build", "-o", out, "-target", "wasm-unknown", "./"+pkg)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s does not build under TinyGo:\n%s", pkg, combined)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestWire_TheGuestDeclaresExactlyTheContractsExports is the phase's named
// done-condition, read off the binary: the steward exports the manifest and
// the wire entry points, and nothing else it declared.
//
// "Nothing else" is exact rather than absolute, and the difference is worth
// stating. Every wasm module the toolchain builds carries a handful of
// exports the toolchain itself emits — the linear memory the host must reach
// to write into the guest at all, the module initializer, and some
// compiler-rt float intrinsics — and an empty main() carries the same ones.
// Those are
// koine/wire's ToolchainExports, named as data so both sides gate on one
// list. What is asserted here is that the guest's OWN declarations are the
// contract's and no more.
func TestWire_TheGuestDeclaresExactlyTheContractsExports(t *testing.T) {
	_, exports := sections(t, buildGuest(t, "fixtures/guest/steward"))

	declared := without(exports, wire.ToolchainExports)
	if strings.Join(declared, ",") != strings.Join(wire.GuestExports, ",") {
		t.Errorf("the guest declares %v, want exactly %v", declared, wire.GuestExports)
	}

	// The toolchain's own set is present and accounted for. If a toolchain
	// upgrade drops or adds one, this fails and the list is corrected —
	// which is the point of writing it down instead of filtering by shape.
	missing := without(wire.ToolchainExports, exports)
	if len(missing) != 0 {
		t.Errorf("ToolchainExports names %v, which this toolchain does not emit — correct the list", missing)
	}
}

// TestWire_TheGuestImportsNothingButTheContractsHostFunctions pins §8's
// claim at the only place it can be pinned: a wasm module can call exactly
// what it imports, and the steward imports a subset of the six this contract
// declares. It is a subset and not the whole set because the linker drops
// what the body never reaches — the steward never gates on the fast beat,
// never logs, and never pulls its own delivery, so those three are simply
// not there.
func TestWire_TheGuestImportsNothingButTheContractsHostFunctions(t *testing.T) {
	imports, _ := sections(t, buildGuest(t, "fixtures/guest/steward"))
	if len(imports) == 0 {
		t.Fatal("the guest imports nothing at all — it cannot even yield")
	}
	for _, imp := range imports {
		module, name, found := strings.Cut(imp, ".")
		// The host registers the same six functions under two names and
		// admits imports from either; a guest may pick one.
		if !found || (module != wire.Module && module != wire.Alias) {
			t.Errorf("the guest imports %q, from outside the two modules the loader admits", imp)
			continue
		}
		if !contains(wire.GuestImports, name) {
			t.Errorf("the guest imports %q, which is not one of the host functions %v", name, wire.GuestImports)
		}
	}
	if !contains(imports, wire.Module+"."+wire.ImportYield) {
		t.Error("the guest does not import yield — emitting is the storage action, and there is no other path")
	}
}

// TestWire_TheSecondPathFixtureDeclaresAnExtraExport keeps the negative
// fixture honest. The engine's refusal test (Sol-Duara-Inc/conduit-go#185)
// is only worth running against a module that genuinely commits the offence;
// a fixture that quietly stopped declaring the extra path would leave that
// test passing against nothing.
func TestWire_TheSecondPathFixtureDeclaresAnExtraExport(t *testing.T) {
	_, exports := sections(t, buildGuest(t, "fixtures/guest/secondpath"))
	declared := without(exports, wire.ToolchainExports)

	extra := without(declared, wire.GuestExports)
	if strings.Join(extra, ",") != "emit" {
		t.Fatalf("the negative fixture declares %v beyond the contract, want exactly [emit]", extra)
	}
	// And "emit" is one of the names the loader refuses outright, so the
	// offence is the one the engine's refusal actually tests.
	if !contains(wire.ForbiddenExports, "emit") {
		t.Error("the negative fixture declares a name the loader does not refuse — it would simply load")
	}
	// It carries the whole conforming surface as well, so the loader is
	// refusing a module it could otherwise have loaded — the refusal is
	// about the extra path and nothing else.
	for _, want := range wire.GuestExports {
		if !contains(declared, want) {
			t.Errorf("the negative fixture is missing %q, so a refusal would not be about the second path", want)
		}
	}
}

// TestWire_TheManifestExportCarriesTheDerivedManifest pins A2 and A3 across
// the boundary at once: what the guest exports under the name the engine
// already reads is the manifest koinegen DERIVED from the station's body,
// byte for byte. Neither guest writes a manifest; both embed one, and both
// embed the same one, because they serve the same station.
func TestWire_TheManifestExportCarriesTheDerivedManifest(t *testing.T) {
	root := repoRoot(t)
	derived, err := os.ReadFile(filepath.Join(root, "cmd", "koinegen", "fixtures", "manifests", "deployment-steward.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, guest := range []string{"steward", "secondpath"} {
		embedded, err := os.ReadFile(filepath.Join(root, "fixtures", "guest", guest, "manifest.json"))
		if err != nil {
			t.Errorf("%s embeds no manifest: %v", guest, err)
			continue
		}
		if !bytes.Equal(embedded, derived) {
			t.Errorf("%s embeds a manifest that is not the derived one — run `go generate ./fixtures/guest/%s`", guest, guest)
		}
	}
}

// sections reads a wasm module's import and export names. It is a few dozen
// lines because the binary format's first two fields are a magic number and
// a version, and everything after that is length-prefixed.
func sections(t *testing.T, module []byte) (imports, exports []string) {
	t.Helper()
	if len(module) < 8 || string(module[:4]) != "\x00asm" {
		t.Fatal("that is not a wasm module")
	}
	r := &wasmReader{data: module, pos: 8, t: t}
	for r.pos < len(r.data) {
		id := r.byteAt()
		size := r.uleb()
		end := r.pos + int(size)
		switch id {
		case 2: // imports
			for n := r.uleb(); n > 0; n-- {
				module := r.name()
				name := r.name()
				r.byteAt() // kind
				r.uleb()   // index or type
				imports = append(imports, module+"."+name)
			}
		case 7: // exports
			for n := r.uleb(); n > 0; n-- {
				name := r.name()
				r.byteAt() // kind
				r.uleb()   // index
				exports = append(exports, name)
			}
		}
		r.pos = end
	}
	sort.Strings(imports)
	sort.Strings(exports)
	return imports, exports
}

type wasmReader struct {
	data []byte
	pos  int
	t    *testing.T
}

func (r *wasmReader) byteAt() byte {
	if r.pos >= len(r.data) {
		r.t.Fatal("the wasm module ends mid-section")
	}
	b := r.data[r.pos]
	r.pos++
	return b
}

func (r *wasmReader) uleb() uint64 {
	var v uint64
	var shift uint
	for {
		b := r.byteAt()
		v |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return v
		}
		shift += 7
	}
}

func (r *wasmReader) name() string {
	n := int(r.uleb())
	if r.pos+n > len(r.data) {
		r.t.Fatal("the wasm module ends mid-name")
	}
	s := string(r.data[r.pos : r.pos+n])
	r.pos += n
	return s
}

func without(all, remove []string) []string {
	var out []string
	for _, s := range all {
		if !contains(remove, s) {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
