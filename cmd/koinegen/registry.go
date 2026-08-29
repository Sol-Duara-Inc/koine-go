package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The schema registry is reverse-DNS namespaces plus inheritance: one file
// per namespace, each naming the namespace it extends. Nothing in a file
// mentions the namespaces that extend IT — downward blindness is a property
// of the registry's shape, not a rule anyone has to remember. A floor
// namespace's generated output is therefore identical whether zero vendors
// or a hundred extend it.

// Registry is the whole registry, loaded and judged.
type Registry struct {
	Namespaces []*Namespace // sorted by namespace name — a stable generation order
	byName     map[string]*Namespace
}

// Namespace is one reverse-DNS namespace: a stratum of the data.
type Namespace struct {
	Namespace  string      `json:"namespace"`
	Package    string      `json:"package"`
	Stratum    string      `json:"stratum"` // floor | vendor | customer
	Extends    string      `json:"extends,omitempty"`
	Doc        string      `json:"doc"`
	Refs       []Ref       `json:"refs,omitempty"`
	Types      []*Type     `json:"types,omitempty"`
	Deliveries []*Delivery `json:"deliveries,omitempty"`

	file   string
	parent *Namespace
}

// Ref is a named address type: a string with a name, so a field says what it
// points at instead of saying "string".
type Ref struct {
	Name string `json:"name"`
	Doc  string `json:"doc"`
}

// Type is one utterance shape at this stratum.
type Type struct {
	Name    string  `json:"name"`
	Event   string  `json:"event"`
	Extends string  `json:"extends,omitempty"` // a type name in the extended namespace
	Doc     string  `json:"doc"`
	Fields  []Field `json:"fields,omitempty"`

	ns     *Namespace
	parent *Type
}

// Field is one key. Type is the small field grammar: string, int, bool,
// outcome, or ref:Name.
type Field struct {
	Name string `json:"name"`
	JSON string `json:"json"`
	Type string `json:"type"`
	Doc  string `json:"doc"`
}

// Delivery is a generated Delivery type: the projected facts a station of
// this stratum is constructed with, plus the verbs its stratum can speak.
type Delivery struct {
	Name  string  `json:"name"`
	Of    string  `json:"of"`
	Doc   string  `json:"doc"`
	Await Await   `json:"await"`
	Verbs []*Verb `json:"verbs,omitempty"`

	ns *Namespace
	of *Type
}

// Await is the selector this delivery projects — the shape a station awaits
// to be handed one of these.
type Await struct {
	Mode   string `json:"mode"` // event | resolved | absent
	Type   string `json:"type"`
	Anchor string `json:"anchor,omitempty"`
}

// Verb is a seat reachable from this delivery.
type Verb struct {
	Name        string    `json:"name"`
	Seat        string    `json:"seat"`
	Tool        string    `json:"tool,omitempty"`
	Connection  string    `json:"connection,omitempty"`
	Permissions []string  `json:"permissions,omitempty"`
	Doc         string    `json:"doc"`
	Intents     []*Intent `json:"intents"`
}

// Intent is one thing that seat can be asked — one spoken exchange.
type Intent struct {
	Name     string  `json:"name"`
	Exchange string  `json:"exchange"`
	Returns  string  `json:"returns"`
	Doc      string  `json:"doc"`
	Params   []Param `json:"params,omitempty"`

	returns *Type
}

// Param is one argument of an intent.
type Param struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// wireYieldTypeKey mirrors wire.YieldTypeKey. The registry cannot import
// koine/wire — a build tool has no business depending on the guest contract
// — so the one key they must agree about is written down in both places and
// pinned by a test.
const wireYieldTypeKey = "type"

var (
	namespacePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z0-9][a-z0-9-]*)+$`)
	identPattern     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	exportedPattern  = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*$`)
	strata           = map[string]bool{"floor": true, "vendor": true, "customer": true}
	awaitModes       = map[string]bool{"event": true, "resolved": true, "absent": true}
)

// LoadRegistry reads every *.json in dir and judges it. It judges in order
// and refuses BY NAME, storing nothing on refusal (A9): a registry that does
// not validate produces no Registry at all, so no half-generated tree can
// exist.
func LoadRegistry(dir string) (*Registry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	reg := &Registry{byName: map[string]*Namespace{}}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("koinegen: no schema registry files in %s", dir)
	}
	for _, name := range files {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		ns := &Namespace{file: name}
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(ns); err != nil {
			return nil, fmt.Errorf("koinegen: %s: %w", name, err)
		}
		if _, dup := reg.byName[ns.Namespace]; dup {
			return nil, fmt.Errorf("koinegen: %s: namespace %q is declared twice", name, ns.Namespace)
		}
		reg.byName[ns.Namespace] = ns
		reg.Namespaces = append(reg.Namespaces, ns)
	}
	sort.Slice(reg.Namespaces, func(i, j int) bool {
		return reg.Namespaces[i].Namespace < reg.Namespaces[j].Namespace
	})
	if err := reg.judge(); err != nil {
		return nil, err
	}
	return reg, nil
}

// Namespace finds a namespace by its reverse-DNS name.
func (r *Registry) Namespace(name string) *Namespace { return r.byName[name] }

// PackageNamed finds the namespace whose generated Go package carries this
// name. Generated packages are named uniquely across the registry, judged at
// load, so this answer is never ambiguous.
func (r *Registry) PackageNamed(pkg string) *Namespace {
	for _, ns := range r.Namespaces {
		if ns.Package == pkg {
			return ns
		}
	}
	return nil
}

func (r *Registry) judge() error {
	packages := map[string]string{}
	for _, ns := range r.Namespaces {
		switch {
		case !namespacePattern.MatchString(ns.Namespace):
			return fmt.Errorf("koinegen: %s: namespace %q is not reverse-DNS", ns.file, ns.Namespace)
		case !identPattern.MatchString(ns.Package):
			return fmt.Errorf("koinegen: %s: package %q is not a Go identifier", ns.file, ns.Package)
		case !strata[ns.Stratum]:
			return fmt.Errorf("koinegen: %s: stratum %q is not floor, vendor or customer", ns.file, ns.Stratum)
		}
		if other, dup := packages[ns.Package]; dup {
			return fmt.Errorf("koinegen: %s: package %q is already generated for namespace %s", ns.file, ns.Package, other)
		}
		packages[ns.Package] = ns.Namespace
	}
	for _, ns := range r.Namespaces {
		if ns.Extends == "" {
			if ns.Stratum != "floor" {
				return fmt.Errorf("koinegen: %s: a %s stratum must extend a namespace", ns.file, ns.Stratum)
			}
			continue
		}
		parent, ok := r.byName[ns.Extends]
		if !ok {
			return fmt.Errorf("koinegen: %s: extends unknown namespace %q", ns.file, ns.Extends)
		}
		ns.parent = parent
	}
	for _, ns := range r.Namespaces {
		if err := ns.judgeCycles(); err != nil {
			return err
		}
	}
	// Linking is a pass of its OWN, before any judgement reads a lineage.
	// It has to be: namespaces are judged in name order, so a namespace
	// that sorts before its ancestor would otherwise walk a one-link chain
	// and the strata below it would be invisible to the shadow and
	// wire-key checks — a guard whose reach depended on the alphabet.
	for _, ns := range r.Namespaces {
		if err := ns.linkTypes(); err != nil {
			return err
		}
	}
	for _, ns := range r.Namespaces {
		if err := ns.judgeTypes(); err != nil {
			return err
		}
	}
	for _, ns := range r.Namespaces {
		if err := ns.judgeDeliveries(); err != nil {
			return err
		}
	}
	for _, ns := range r.Namespaces {
		if err := ns.judgeIdentifiers(); err != nil {
			return err
		}
	}
	return nil
}

func (ns *Namespace) judgeCycles() error {
	seen := map[string]bool{ns.Namespace: true}
	for p := ns.parent; p != nil; p = p.parent {
		if seen[p.Namespace] {
			return fmt.Errorf("koinegen: %s: namespace lineage loops at %q", ns.file, p.Namespace)
		}
		seen[p.Namespace] = true
	}
	return nil
}

// Lineage is this namespace and everything it extends, deepest stratum first.
func (ns *Namespace) Lineage() []string {
	var out []string
	for p := ns; p != nil; p = p.parent {
		out = append(out, p.Namespace)
	}
	return out
}

// Parent is the namespace this one extends, or nil at the floor.
func (ns *Namespace) Parent() *Namespace { return ns.parent }

// TypeNamed finds a type declared in this namespace.
func (ns *Namespace) TypeNamed(name string) *Type {
	for _, t := range ns.Types {
		if t.Name == name {
			return t
		}
	}
	return nil
}

// RefOwner walks the lineage for the namespace that declares this ref.
func (ns *Namespace) RefOwner(name string) *Namespace {
	for p := ns; p != nil; p = p.parent {
		for _, ref := range p.Refs {
			if ref.Name == name {
				return p
			}
		}
	}
	return nil
}

// linkTypes binds each type to its declaring namespace and to the type it
// extends. It runs for every namespace before any of them is judged, so a
// check that walks a lineage always walks the whole one.
func (ns *Namespace) linkTypes() error {
	names := map[string]bool{}
	for _, t := range ns.Types {
		t.ns = ns
		if !exportedPattern.MatchString(t.Name) {
			return fmt.Errorf("koinegen: %s: type %q must be an exported Go identifier", ns.file, t.Name)
		}
		if names[t.Name] {
			return fmt.Errorf("koinegen: %s: type %q is declared twice", ns.file, t.Name)
		}
		names[t.Name] = true
		if t.Extends == "" {
			continue
		}
		if ns.parent == nil {
			return fmt.Errorf("koinegen: %s: type %q extends %q but the namespace extends nothing", ns.file, t.Name, t.Extends)
		}
		parent := ns.parent.TypeNamed(t.Extends)
		if parent == nil {
			return fmt.Errorf("koinegen: %s: type %q extends unknown type %s.%s", ns.file, t.Name, ns.parent.Namespace, t.Extends)
		}
		t.parent = parent
	}
	return nil
}

func (ns *Namespace) judgeTypes() error {
	for _, ref := range ns.Refs {
		if !exportedPattern.MatchString(ref.Name) {
			return fmt.Errorf("koinegen: %s: ref %q must be an exported Go identifier", ns.file, ref.Name)
		}
	}
	for _, t := range ns.Types {
		if t.Event == "" {
			return fmt.Errorf("koinegen: %s: type %q declares no event type", ns.file, t.Name)
		}
		fields := map[string]bool{}
		keys := map[string]bool{}
		for _, f := range t.AllFields() {
			if fields[f.Name] {
				return fmt.Errorf("koinegen: %s: type %q shadows inherited field %q — a stratum extends, it never overwrites", ns.file, t.Name, f.Name)
			}
			fields[f.Name] = true
			if keys[f.JSON] {
				return fmt.Errorf("koinegen: %s: type %q reuses wire key %q", ns.file, t.Name, f.JSON)
			}
			keys[f.JSON] = true
		}
		for _, f := range t.Fields {
			if !exportedPattern.MatchString(f.Name) {
				return fmt.Errorf("koinegen: %s: %s.%s must be an exported Go identifier", ns.file, t.Name, f.Name)
			}
			if f.JSON == "" {
				return fmt.Errorf("koinegen: %s: %s.%s declares no wire key", ns.file, t.Name, f.Name)
			}
			// The yield frame writes the event type beside the object's
			// own keys, flat, because the host stores those very bytes
			// as the payload — a wrapper would put the wire's paperwork
			// in the record instead of the station's speech. A stratum
			// key spelled "type" would collide with it, so it is
			// refused here rather than discovered as a corrupted
			// emission.
			if f.JSON == wireYieldTypeKey {
				return fmt.Errorf("koinegen: %s: %s.%s uses the wire key %q, which the yield frame writes the event type into — a stratum may not spell a key that way", ns.file, t.Name, f.Name, wireYieldTypeKey)
			}
			if _, err := ns.resolveFieldType(f.Type); err != nil {
				return fmt.Errorf("koinegen: %s: %s.%s: %w", ns.file, t.Name, f.Name, err)
			}
		}
	}
	return nil
}

func (ns *Namespace) judgeDeliveries() error {
	names := map[string]bool{}
	for _, d := range ns.Deliveries {
		d.ns = ns
		switch {
		case !exportedPattern.MatchString(d.Name):
			return fmt.Errorf("koinegen: %s: delivery %q must be an exported Go identifier", ns.file, d.Name)
		case !strings.HasSuffix(d.Name, "Delivery"):
			return fmt.Errorf("koinegen: %s: delivery %q must end in Delivery", ns.file, d.Name)
		case d.Name == "Delivery":
			return fmt.Errorf("koinegen: %s: a delivery needs a name before the suffix", ns.file)
		case names[d.Name]:
			return fmt.Errorf("koinegen: %s: delivery %q is declared twice", ns.file, d.Name)
		}
		names[d.Name] = true
		d.of = ns.TypeNamed(d.Of)
		if d.of == nil {
			return fmt.Errorf("koinegen: %s: delivery %q projects unknown type %q", ns.file, d.Name, d.Of)
		}
		if !awaitModes[d.Await.Mode] {
			return fmt.Errorf("koinegen: %s: delivery %q awaits in unknown mode %q", ns.file, d.Name, d.Await.Mode)
		}
		if d.Await.Type == "" {
			return fmt.Errorf("koinegen: %s: delivery %q awaits nothing", ns.file, d.Name)
		}
		seats := map[string]bool{}
		for _, v := range d.Verbs {
			switch {
			case !exportedPattern.MatchString(v.Name):
				return fmt.Errorf("koinegen: %s: verb %q must be an exported Go identifier", ns.file, v.Name)
			case v.Seat == "":
				return fmt.Errorf("koinegen: %s: verb %q names no seat", ns.file, v.Name)
			case seats[v.Name]:
				return fmt.Errorf("koinegen: %s: verb %q is declared twice on %s", ns.file, v.Name, d.Name)
			case len(v.Intents) == 0:
				return fmt.Errorf("koinegen: %s: seat %q can be asked nothing", ns.file, v.Seat)
			}
			seats[v.Name] = true
			for _, in := range v.Intents {
				switch {
				case !exportedPattern.MatchString(in.Name):
					return fmt.Errorf("koinegen: %s: intent %q must be an exported Go identifier", ns.file, in.Name)
				case in.Exchange == "":
					return fmt.Errorf("koinegen: %s: intent %s.%s names no exchange", ns.file, v.Name, in.Name)
				}
				in.returns = ns.TypeNamed(in.Returns)
				if in.returns == nil {
					return fmt.Errorf("koinegen: %s: intent %s.%s returns unknown type %q", ns.file, v.Name, in.Name, in.Returns)
				}
				for _, p := range in.Params {
					if !identPattern.MatchString(p.Name) {
						return fmt.Errorf("koinegen: %s: intent %s.%s parameter %q is not a Go identifier", ns.file, v.Name, in.Name, p.Name)
					}
					if _, err := scalarKind(p.Type); err != nil {
						return fmt.Errorf("koinegen: %s: intent %s.%s parameter %q: %w", ns.file, v.Name, in.Name, p.Name, err)
					}
				}
			}
		}
	}
	return nil
}

// AllFields is the type's whole kingdom: inherited fields first, in lineage
// order, then its own. This is the flattening the wire already performs —
// the host projects to a lineage and hands the guest one flat object.
func (t *Type) AllFields() []Field {
	if t.parent == nil {
		return append([]Field(nil), t.Fields...)
	}
	return append(t.parent.AllFields(), t.Fields...)
}

// Owner is the namespace that declares this type — the owning stratum, and
// the only place its seeding constructor is generated.
func (t *Type) Owner() *Namespace { return t.ns }

// Parent is the type this one extends, or nil at the floor.
func (t *Type) Parent() *Type { return t.parent }

// FieldOwner walks the lineage for the type that declares this field, so a
// generated seeder can name the field's type in the package that owns it.
func (t *Type) FieldOwner(name string) *Type {
	for p := t; p != nil; p = p.parent {
		for _, f := range p.Fields {
			if f.Name == name {
				return p
			}
		}
	}
	return nil
}

// Projects is the type this delivery projects.
func (d *Delivery) Projects() *Type { return d.of }

// Owner is the namespace the delivery is generated into.
func (d *Delivery) Owner() *Namespace { return d.ns }

// AwaitFunc is the name of the generated selector constructor: the delivery
// name without its Delivery suffix. deployment.ResolvedDelivery is awaited
// by writing deployment.Resolved().
func (d *Delivery) AwaitFunc() string { return strings.TrimSuffix(d.Name, "Delivery") }

// ReturnType is the type an intent's handle materializes.
func (in *Intent) ReturnType() *Type { return in.returns }

// errUnknownFieldType names the grammar rather than pointing at it.
var errUnknownFieldType = errors.New(`field type must be one of string, int, bool, outcome, ref:Name`)

func scalarKind(t string) (string, error) {
	switch t {
	case "string", "int", "bool", "outcome":
		return t, nil
	}
	return "", errUnknownFieldType
}

// resolveFieldType checks a field type against the grammar and, for a ref,
// against the lineage that must already declare it.
func (ns *Namespace) resolveFieldType(t string) (string, error) {
	if name, ok := strings.CutPrefix(t, "ref:"); ok {
		if !exportedPattern.MatchString(name) {
			return "", fmt.Errorf("ref %q must be an exported Go identifier", name)
		}
		if ns.RefOwner(name) == nil {
			return "", fmt.Errorf("ref %q is declared by no namespace in the lineage", name)
		}
		return t, nil
	}
	return scalarKind(t)
}

// judgeIdentifiers refuses a namespace whose generated package would declare
// one name twice. Codegen derives package-scope identifiers from several
// independent places in the registry — a type, its seeding constructor, a
// delivery, the selector constructor named after it, a seat type, a handle —
// and none of those places can see the others. Collecting every name the
// package WILL emit and judging the set is the only place the collision is
// visible before the Go compiler finds it.
func (ns *Namespace) judgeIdentifiers() error {
	declared := map[string]string{}
	claim := func(ident, by string) error {
		if prior, taken := declared[ident]; taken {
			return fmt.Errorf("koinegen: %s: package %s would declare %q twice — %s and %s", ns.file, ns.Package, ident, prior, by)
		}
		declared[ident] = by
		return nil
	}
	for _, ref := range ns.Refs {
		if err := claim(ref.Name, "the ref "+ref.Name); err != nil {
			return err
		}
	}
	for _, t := range ns.Types {
		if err := claim(t.Name, "the type "+t.Name); err != nil {
			return err
		}
		if err := claim("Seed"+t.Name, "the seeding constructor for "+t.Name); err != nil {
			return err
		}
	}
	for _, d := range ns.Deliveries {
		if err := claim(d.Name, "the delivery "+d.Name); err != nil {
			return err
		}
		if err := claim(d.AwaitFunc(), "the selector constructor for "+d.Name); err != nil {
			return err
		}
		for _, v := range d.Verbs {
			if err := claim(seatTypeName(v), "the seat type for "+d.Name+"."+v.Name); err != nil {
				return err
			}
			for _, in := range v.Intents {
				ident := handleTypeName(in.ReturnType())
				if declared[ident] == "" {
					declared[ident] = "the handle for " + in.ReturnType().Name
					continue
				}
				// One handle type serves every intent that answers with
				// the same shape; that is sharing, not collision.
				if !strings.HasPrefix(declared[ident], "the handle for ") {
					return fmt.Errorf("koinegen: %s: package %s would declare %q twice — %s and the handle for %s",
						ns.file, ns.Package, ident, declared[ident], in.ReturnType().Name)
				}
			}
		}
	}
	// Generated files import the packages of the strata below, so a local
	// name that shadows one of them is refused here too.
	for p := ns.parent; p != nil; p = p.parent {
		if prior, taken := declared[p.Package]; taken {
			return fmt.Errorf("koinegen: %s: package %s declares %q (%s), which shadows the imported package for %s",
				ns.file, ns.Package, p.Package, prior, p.Namespace)
		}
	}
	return nil
}
