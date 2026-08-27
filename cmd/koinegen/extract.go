package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/sol-duara-inc/koine-go/koine/manifest"
)

// The manifest is derived from the code and from nowhere else (A3). The
// extractor reads the station's own source — the identity it returns, the
// selectors it lists, the utterances its body yields, the exchanges it
// speaks and how it consumes them — and the registry, which is the only
// other thing the station itself was generated against. There is no third
// input, and in particular there is no hand-written manifest to merge with:
// a declaration a human typed could lie about the body it describes, so this
// tool refuses one by name rather than reading it.

// handWrittenNames are the files a station author might be tempted to write
// a manifest into. Finding one is a refusal, not a fallback.
var handWrittenNames = []string{"manifest.json", "koine.manifest.json", "station.manifest.json"}

// Extract reads every station declared in dir and derives its manifest.
// It judges in order and refuses BY NAME, returning nothing on refusal (A9):
// a station that cannot be derived produces no partial manifest anywhere.
func Extract(reg *Registry, dir string) (map[string]manifest.Manifest, error) {
	for _, name := range handWrittenNames {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return nil, fmt.Errorf("koinegen manifest: %s holds a hand-written %s — manifests are derived from code, never read from one; delete it", dir, name)
		}
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		return nil, err
	}
	out := map[string]manifest.Manifest{}
	names := make([]string, 0, len(pkgs))
	for name := range pkgs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		x := &extractor{reg: reg, fset: fset, pkg: pkgs[name]}
		if err := x.readPackage(); err != nil {
			return nil, err
		}
		for station, m := range x.manifests {
			out[station] = m
		}
	}
	return out, nil
}

// extractor holds one package's worth of reading.
type extractor struct {
	reg  *Registry
	fset *token.FileSet
	pkg  *ast.Package

	koineLocal    string
	selectorLocal string
	strata        map[string]*Namespace // local package name -> namespace
	methods       map[string]map[string]*ast.FuncDecl
	bases         map[string]string // station type -> stratum
	manifests     map[string]manifest.Manifest
}

func (x *extractor) readPackage() error {
	x.strata = map[string]*Namespace{}
	x.methods = map[string]map[string]*ast.FuncDecl{}
	x.bases = map[string]string{}
	x.manifests = map[string]manifest.Manifest{}

	files := make([]string, 0, len(x.pkg.Files))
	for path := range x.pkg.Files {
		files = append(files, path)
	}
	sort.Strings(files)

	for _, path := range files {
		x.readImports(x.pkg.Files[path])
	}
	if x.koineLocal == "" {
		return fmt.Errorf("koinegen manifest: package %s imports no koine — there is no station here", x.pkg.Name)
	}
	for _, path := range files {
		x.readDecls(x.pkg.Files[path])
	}

	stations := make([]string, 0, len(x.bases))
	for name := range x.bases {
		stations = append(stations, name)
	}
	sort.Strings(stations)
	for _, name := range stations {
		m, err := x.station(name)
		if err != nil {
			return err
		}
		x.manifests[name] = m
	}
	return nil
}

func (x *extractor) readImports(f *ast.File) {
	for _, spec := range f.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		local := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			local = spec.Name.Name
		}
		switch path {
		case koinePath:
			x.koineLocal = local
			continue
		case selectorPath:
			x.selectorLocal = local
			continue
		}
		last := path[strings.LastIndex(path, "/")+1:]
		if ns := x.reg.PackageNamed(last); ns != nil && strings.HasSuffix(path, "/"+last) {
			x.strata[local] = ns
		}
	}
}

func (x *extractor) readDecls(f *ast.File) {
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if stratum := x.stratumOf(st); stratum != "" {
					x.bases[ts.Name.Name] = stratum
				}
			}
		case *ast.FuncDecl:
			recv := receiverType(d)
			if recv == "" {
				continue
			}
			if x.methods[recv] == nil {
				x.methods[recv] = map[string]*ast.FuncDecl{}
			}
			x.methods[recv][d.Name.Name] = d
		}
	}
}

// stratumOf reads plane discipline straight off the embedding. There is no
// permission bit to read: the stratum IS which base the author embedded, and
// which verbs therefore exist from that position.
func (x *extractor) stratumOf(st *ast.StructType) string {
	for _, f := range st.Fields.List {
		if len(f.Names) != 0 {
			continue
		}
		sel, ok := f.Type.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if ident, ok := sel.X.(*ast.Ident); !ok || ident.Name != x.koineLocal {
			continue
		}
		switch sel.Sel.Name {
		case "ObserverBase":
			return "observer"
		case "ExecutionBase":
			return "execution"
		}
	}
	return ""
}

func receiverType(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) != 1 {
		return ""
	}
	t := d.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if ident, ok := t.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// station derives one station's manifest, judging in §7's order: identity,
// then selectors, then seats, then plane, then topology.
func (x *extractor) station(name string) (manifest.Manifest, error) {
	var m manifest.Manifest
	methods := x.methods[name]
	for _, needed := range []string{"Identity", "Awaits", "Complete", "Resolve"} {
		if methods[needed] == nil {
			return m, fmt.Errorf("koinegen manifest: %s embeds a stratum base but declares no %s — it is not a station, and half a station is refused whole", name, needed)
		}
	}
	claim, err := x.identity(name, methods["Identity"])
	if err != nil {
		return m, err
	}
	awaits, err := x.awaits(name, methods["Awaits"])
	if err != nil {
		return m, err
	}
	body, err := x.resolveBody(name, methods["Resolve"])
	if err != nil {
		return m, err
	}
	events := make([]manifest.Event, 0, len(body.emits))
	for _, e := range body.emits {
		events = append(events, manifest.Event{
			Type:           e.Type,
			Direction:      "emit",
			Reconciliation: &manifest.Reconciliation{Declared: false},
		})
	}
	return manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Kind:          "station",
		Claim:         claim,
		Events:        events,
		Koine: manifest.Koine{
			Stratum:   x.bases[name],
			Lineage:   body.lineage,
			Complete:  x.complete(methods["Complete"]),
			Awaits:    awaits,
			Emits:     body.emits,
			Exchanges: body.exchanges,
			Seats:     body.seats,
		},
	}, nil
}

// isKoineType asks whether an expression names koine.<name> under whatever
// local name the file imported the package as.
func (x *extractor) isKoineType(expr ast.Expr, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == x.koineLocal
}

func (x *extractor) identity(station string, fn *ast.FuncDecl) (manifest.Claim, error) {
	var claim manifest.Claim
	var found bool
	ast.Inspect(fn, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !x.isKoineType(lit.Type, "Identity") {
			return true
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, _ := kv.Key.(*ast.Ident)
			value, ok := stringLit(kv.Value)
			if key == nil || !ok {
				continue
			}
			switch key.Name {
			case "Group":
				claim.Group = value
			case "Author":
				claim.Author = value
			case "Name":
				claim.Name = value
			}
		}
		found = true
		return false
	})
	switch {
	case !found:
		return claim, fmt.Errorf("koinegen manifest: %s.Identity returns no koine.Identity literal — the claim must be readable in the source that carries it", station)
	case claim.Group == "" || claim.Author == "" || claim.Name == "":
		return claim, fmt.Errorf("koinegen manifest: %s.Identity is incomplete (%q/%q/%q) — a claim is the whole tuple or it is refused", station, claim.Group, claim.Author, claim.Name)
	}
	return claim, nil
}

func (x *extractor) awaits(station string, fn *ast.FuncDecl) ([]manifest.Await, error) {
	var list *ast.CallExpr
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || list != nil {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "List" {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == x.selectorLocal {
				list = call
				return false
			}
		}
		return true
	})
	if list == nil {
		return nil, fmt.Errorf("koinegen manifest: %s.Awaits does not return selector.List(...) — the expected graph is the routing table and must be readable here", station)
	}
	out := make([]manifest.Await, 0, len(list.Args))
	for _, arg := range list.Args {
		a, err := x.await(station, arg)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("koinegen manifest: %s awaits nothing — a station with no expectation receives nothing and routes nothing", station)
	}
	return out, nil
}

// await reads one selector expression. Anchors ride as .At(...) on the way
// out, so they are peeled before the base shape is read.
func (x *extractor) await(station string, expr ast.Expr) (manifest.Await, error) {
	var a manifest.Await
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return a, fmt.Errorf("koinegen manifest: %s awaits %s, which is not a selector call", station, x.src(expr))
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return a, fmt.Errorf("koinegen manifest: %s awaits %s, which names no selector", station, x.src(expr))
	}
	if sel.Sel.Name == "At" {
		anchor, ok := stringLit(call.Args[0])
		if !ok {
			return a, fmt.Errorf("koinegen manifest: %s anchors on a value that is not a literal in %s", station, x.src(expr))
		}
		inner, err := x.await(station, sel.X)
		if err != nil {
			return a, err
		}
		inner.Anchor = anchor
		inner.Hash = awaitHash(inner)
		return inner, nil
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return a, fmt.Errorf("koinegen manifest: %s awaits %s, which names no package", station, x.src(expr))
	}
	if pkg.Name == x.selectorLocal {
		return x.grammarAwait(station, sel.Sel.Name, call)
	}
	ns := x.strata[pkg.Name]
	if ns == nil {
		return a, fmt.Errorf("koinegen manifest: %s awaits %s, whose package is not a generated stratum", station, x.src(expr))
	}
	for _, d := range ns.Deliveries {
		if d.AwaitFunc() != sel.Sel.Name {
			continue
		}
		a = manifest.Await{Type: d.Await.Type, Mode: d.Await.Mode, Anchor: d.Await.Anchor}
		a.Hash = awaitHash(a)
		return a, nil
	}
	return a, fmt.Errorf("koinegen manifest: %s awaits %s, which the registry declares for no delivery in %s", station, x.src(expr), ns.Namespace)
}

func (x *extractor) grammarAwait(station, fn string, call *ast.CallExpr) (manifest.Await, error) {
	var a manifest.Await
	switch fn {
	case "Event", "Resolved":
		shape, ok := stringLit(call.Args[0])
		if !ok {
			return a, fmt.Errorf("koinegen manifest: %s awaits a shape that is not a literal", station)
		}
		a = manifest.Await{Type: shape, Mode: strings.ToLower(fn)}
	case "Absent":
		inner, err := x.await(station, call.Args[0])
		if err != nil {
			return a, err
		}
		a = inner
		a.Mode = string(selectorModeAbsent)
	default:
		return a, fmt.Errorf("koinegen manifest: %s awaits through selector.%s, which is not part of the grammar", station, fn)
	}
	a.Hash = awaitHash(a)
	return a, nil
}

// selectorModeAbsent mirrors selector.ModeAbsent without importing the
// grammar into a build tool that only needs its spelling.
const selectorModeAbsent = "absent"

// awaitHash content-pins a selector: the door binds the pin to the
// expression store the registration cites, so a selector cannot be edited
// after the fact without the manifest saying so.
func awaitHash(a manifest.Await) string {
	sum := sha256.Sum256([]byte(a.Mode + "\x1f" + a.Type + "\x1f" + a.Anchor))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (x *extractor) complete(fn *ast.FuncDecl) string {
	found := "authored"
	ast.Inspect(fn, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == x.koineLocal && sel.Sel.Name == "DefaultAllAwaited" {
			found = "all-awaited"
			return false
		}
		return true
	})
	return found
}

// resolved is what one Resolve body says about itself.
type resolved struct {
	lineage   []string
	emits     []manifest.Emit
	exchanges []manifest.Speaks
	seats     []manifest.SeatNeed
}

func (x *extractor) resolveBody(station string, fn *ast.FuncDecl) (resolved, error) {
	var out resolved
	if len(fn.Type.Params.List) != 2 {
		return out, fmt.Errorf("koinegen manifest: %s.Resolve does not take (koine.Delivery, koine.Yield)", station)
	}
	delivery, ns, err := x.delivery(station, fn)
	if err != nil {
		return out, err
	}
	out.lineage = ns.Lineage()

	b, err := x.newBody(station, fn, delivery)
	if err != nil {
		return out, err
	}

	seen := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == b.yieldParam && len(call.Args) == 1 {
			emit, ok := x.emit(call.Args[0])
			if !ok {
				return true
			}
			if !seen[emit.Go] {
				seen[emit.Go] = true
				out.emits = append(out.emits, emit)
			}
		}
		return true
	})
	if len(out.emits) == 0 {
		return out, fmt.Errorf("koinegen manifest: %s.Resolve yields nothing — a station that never speaks has no reason to be awaited", station)
	}

	spoken, err := x.spoken(b)
	if err != nil {
		return out, err
	}
	out.exchanges = spoken
	out.seats = seatNeeds(delivery, spoken)
	return out, nil
}

// delivery finds the stratum type the body asserts itself into. The
// assertion is the station's declaration of which kingdom it stands in, and
// the lineage follows from it.
func (x *extractor) delivery(station string, fn *ast.FuncDecl) (*Delivery, *Namespace, error) {
	var found *Delivery
	var ns *Namespace
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assert, ok := n.(*ast.TypeAssertExpr)
		if !ok || assert.Type == nil || found != nil {
			return true
		}
		sel, ok := assert.Type.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		candidate := x.strata[pkg.Name]
		if candidate == nil {
			return true
		}
		for _, d := range candidate.Deliveries {
			if d.Name == sel.Sel.Name {
				found, ns = d, candidate
				return false
			}
		}
		return true
	})
	if found == nil {
		return nil, nil, fmt.Errorf("koinegen manifest: %s.Resolve never names its delivery type — the stratum a body stands in must be readable in the body", station)
	}
	return found, ns, nil
}

func (x *extractor) emit(expr ast.Expr) (manifest.Emit, bool) {
	var e manifest.Emit
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return e, false
	}
	sel, ok := lit.Type.(*ast.SelectorExpr)
	if !ok {
		return e, false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return e, false
	}
	ns := x.strata[pkg.Name]
	if ns == nil {
		return e, false
	}
	t := ns.TypeNamed(sel.Sel.Name)
	if t == nil {
		return e, false
	}
	return manifest.Emit{Go: pkg.Name + "." + t.Name, Type: t.Event, Namespace: ns.Namespace}, true
}
