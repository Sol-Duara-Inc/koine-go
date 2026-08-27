package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"sort"
	"strconv"

	"github.com/sol-duara-inc/koine-go/koine/manifest"
)

// Consumption is read from usage, exactly as §6 says it compiles from usage.
// The three patterns are not three APIs; they are three ways of treating one
// handle:
//
//   - Value() reached on the handle          → inline
//   - koine.Detach reached on the handle     → detached
//   - neither                                → concurrent, the default
//
// Concurrent is the default because silence must never mean "ungated". A
// station that speaks and walks away has still gated its own completion on
// the answer; only a written Detach releases it.
//
// WHERE THE READING CANNOT BE CERTAIN, THE STATION IS REFUSED BY NAME. That
// is this repository's own law (A9: everything validates or is refused), and
// it binds hardest here: the manifest is a contract the engine mints
// coordinates and budget from, so a guess that lands in one is worse than no
// manifest at all. The analyzer therefore follows each handle through its
// binding — honouring how many times the name is written and every place it
// is read — and refuses reassignment, shadowing, a handle or a seat or the
// delivery handed to something it cannot follow, and the contradiction of
// consuming and detaching the same exchange.

// body is one Resolve, read whole: the tree, the parent of every node in it,
// and every position that binds a name.
type body struct {
	station  string
	fn       *ast.FuncDecl
	delivery *Delivery

	parents map[ast.Node]ast.Node
	writes  map[string][]*ast.Ident

	deliveryParam string // the koine.Delivery argument
	yieldParam    string // the koine.Yield argument
	deliveryIdent string // what the type assertion bound the delivery to
	seatIdents    map[string]*Verb
}

// spokenCall is one exchange found in a body.
type spokenCall struct {
	verb   *Verb
	intent *Intent
	node   *ast.CallExpr
}

func (b *body) refuse(format string, args ...any) error {
	return fmt.Errorf("koinegen manifest: "+b.station+".Resolve "+format, args...)
}

// newBody reads the shape of one Resolve before anything is derived from it.
func (x *extractor) newBody(station string, fn *ast.FuncDecl, d *Delivery) (*body, error) {
	b := &body{
		station:    station,
		fn:         fn,
		delivery:   d,
		parents:    parentsOf(fn),
		writes:     collectWrites(fn),
		seatIdents: map[string]*Verb{},
	}
	if names := fn.Type.Params.List[0].Names; len(names) == 1 {
		b.deliveryParam = names[0].Name
	}
	if names := fn.Type.Params.List[1].Names; len(names) == 1 {
		b.yieldParam = names[0].Name
	}
	b.deliveryIdent = x.assertedIdent(b)
	if err := x.judgeEscapes(b); err != nil {
		return nil, err
	}
	if err := x.bindSeats(b); err != nil {
		return nil, err
	}
	return b, nil
}

// parentsOf links every node under root to the node that holds it. The
// consumption reading is entirely a question of what encloses what, and
// guessing at that from a second traversal is how an analyzer starts
// answering questions it was not asked.
func parentsOf(root ast.Node) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}
		if len(stack) > 0 {
			parents[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})
	return parents
}

// collectWrites records every position that binds or assigns a name —
// parameters, short declarations, var specs, range variables and closure
// parameters alike. A name written more than once is a name this analyzer
// will not reason about.
func collectWrites(fn *ast.FuncDecl) map[string][]*ast.Ident {
	writes := map[string][]*ast.Ident{}
	add := func(id *ast.Ident) {
		if id == nil || id.Name == "_" {
			return
		}
		writes[id.Name] = append(writes[id.Name], id)
	}
	fields := func(list *ast.FieldList) {
		if list == nil {
			return
		}
		for _, f := range list.List {
			for _, n := range f.Names {
				add(n)
			}
		}
	}
	fields(fn.Recv)
	fields(fn.Type.Params)
	fields(fn.Type.Results)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					add(id)
				}
			}
		case *ast.ValueSpec:
			for _, id := range s.Names {
				add(id)
			}
		case *ast.RangeStmt:
			if s.Tok == token.DEFINE {
				if id, ok := s.Key.(*ast.Ident); ok {
					add(id)
				}
				if id, ok := s.Value.(*ast.Ident); ok {
					add(id)
				}
			}
		case *ast.FuncLit:
			fields(s.Type.Params)
			fields(s.Type.Results)
		}
		return true
	})
	return writes
}

// readsOf is every use of a name that is not one of its bindings. Selector
// members and composite-literal keys are not uses of a local name, however
// they are spelled.
func (b *body) readsOf(name string) []*ast.Ident {
	written := map[*ast.Ident]bool{}
	for _, id := range b.writes[name] {
		written[id] = true
	}
	var out []*ast.Ident
	ast.Inspect(b.fn, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || id.Name != name || written[id] {
			return true
		}
		switch p := b.parents[id].(type) {
		case *ast.SelectorExpr:
			if p.Sel == id {
				return true // a member name, not this local
			}
		case *ast.KeyValueExpr:
			if p.Key == id {
				return true // a field name in a literal
			}
		}
		out = append(out, id)
		return true
	})
	return out
}

// assertedIdent is the name the body gave its delivery, when it gave it one.
func (x *extractor) assertedIdent(b *body) string {
	name := ""
	ast.Inspect(b.fn.Body, func(n ast.Node) bool {
		if name != "" {
			return false
		}
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range assign.Rhs {
			if _, ok := rhs.(*ast.TypeAssertExpr); !ok || i >= len(assign.Lhs) {
				continue
			}
			if id, ok := assign.Lhs[i].(*ast.Ident); ok && id.Name != "_" {
				name = id.Name
				return false
			}
		}
		return true
	})
	return name
}

// judgeEscapes refuses a body whose speech could leave it. koinegen reads
// ONE method; a Resolve that hands its delivery, a seat, or its yield to a
// helper puts speech somewhere the extractor never looks, and speech that
// never reaches the manifest can never be refused at registration by name —
// which defeats §7.3 outright. Factoring a long Resolve is ordinary Go, so
// this refuses it out loud rather than dropping it in silence.
func (x *extractor) judgeEscapes(b *body) error {
	if b.deliveryParam != "" {
		for _, id := range b.readsOf(b.deliveryParam) {
			if _, ok := b.parents[id].(*ast.TypeAssertExpr); !ok {
				return b.refuse("hands its koine.Delivery argument to %s — the analyzer reads this body and no other, so speech that leaves it never reaches the manifest, and an undeclared seat can never be refused by name. Assert the delivery here and keep the body whole.", x.enclosing(b, id))
			}
		}
	}
	if b.yieldParam != "" {
		for _, id := range b.readsOf(b.yieldParam) {
			if !isCallee(b, id) {
				return b.refuse("hands its koine.Yield argument to %s — speech spoken elsewhere never reaches the manifest. Yield in this body.", x.enclosing(b, id))
			}
		}
	}
	if b.deliveryIdent == "" {
		return nil
	}
	if len(b.writes[b.deliveryIdent]) != 1 {
		return b.refuse("binds %q more than once — the analyzer follows one delivery per name.", b.deliveryIdent)
	}
	for _, id := range b.readsOf(b.deliveryIdent) {
		sel, ok := b.parents[id].(*ast.SelectorExpr)
		if !ok || sel.X != ast.Expr(id) {
			return b.refuse("hands its delivery %q to %s — the analyzer reads this body and no other, so speech that leaves it never reaches the manifest. Keep the body whole.", b.deliveryIdent, x.enclosing(b, id))
		}
	}
	return nil
}

// bindSeats reads `seat := d.History()` and holds the analyzer to the same
// discipline it holds a handle to: bound once, and used only to speak the
// intents that seat answers.
func (x *extractor) bindSeats(b *body) error {
	var bound []*ast.Ident
	ast.Inspect(b.fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range s.Rhs {
				if v := x.verbCall(b, rhs); v != nil && i < len(s.Lhs) {
					if id, ok := s.Lhs[i].(*ast.Ident); ok && id.Name != "_" {
						b.seatIdents[id.Name] = v
						bound = append(bound, id)
					}
				}
			}
		case *ast.ValueSpec:
			for i, val := range s.Values {
				if v := x.verbCall(b, val); v != nil && i < len(s.Names) {
					if id := s.Names[i]; id.Name != "_" {
						b.seatIdents[id.Name] = v
						bound = append(bound, id)
					}
				}
			}
		}
		return true
	})
	for _, id := range bound {
		verb := b.seatIdents[id.Name]
		if len(b.writes[id.Name]) != 1 {
			return b.refuse("binds the seat %q more than once — the analyzer follows one seat per name. Give each seat its own name.", id.Name)
		}
		for _, read := range b.readsOf(id.Name) {
			sel, ok := b.parents[read].(*ast.SelectorExpr)
			if !ok || sel.X != ast.Expr(read) || !isCallee(b, sel) || intentNamed(verb, sel.Sel.Name) == nil {
				return b.refuse("hands the %q seat to %s — speak the seat's intents in this body, or the seat never reaches the manifest and registration can never refuse it by name.", verb.Seat, x.enclosing(b, read))
			}
		}
	}
	// A seat reached but neither spoken at nor bound has gone somewhere the
	// analyzer cannot follow.
	var escaped error
	ast.Inspect(b.fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || escaped != nil {
			return escaped == nil
		}
		verb := x.verbCall(b, call)
		if verb == nil {
			return true
		}
		switch p := b.parents[call].(type) {
		case *ast.SelectorExpr:
			if p.X == ast.Expr(call) {
				return true // an intent is spoken on it
			}
		case *ast.AssignStmt, *ast.ValueSpec:
			return true // bound, and judged above
		}
		escaped = b.refuse("hands the %q seat to %s — speak the seat's intents in this body, or the seat never reaches the manifest and registration can never refuse it by name.", verb.Seat, x.enclosing(b, call))
		return false
	})
	return escaped
}

// verbCall recognises `<delivery>.History()` — the delivery named by the
// ident the assertion bound, or asserted inline.
func (x *extractor) verbCall(b *body, expr ast.Expr) *Verb {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	switch base := sel.X.(type) {
	case *ast.Ident:
		if b.deliveryIdent == "" || base.Name != b.deliveryIdent {
			return nil
		}
	case *ast.TypeAssertExpr:
	default:
		return nil
	}
	for _, v := range b.delivery.Verbs {
		if v.Name == sel.Sel.Name {
			return v
		}
	}
	return nil
}

// seatOf resolves whatever an intent was spoken at: a verb call, or a name
// bound to one.
func (x *extractor) seatOf(b *body, expr ast.Expr) *Verb {
	if v := x.verbCall(b, expr); v != nil {
		return v
	}
	if id, ok := expr.(*ast.Ident); ok {
		return b.seatIdents[id.Name]
	}
	return nil
}

func intentNamed(v *Verb, name string) *Intent {
	for _, in := range v.Intents {
		if in.Name == name {
			return in
		}
	}
	return nil
}

// isCallee reports whether expr is the thing being called, rather than an
// argument to a call.
func isCallee(b *body, expr ast.Node) bool {
	call, ok := b.parents[expr].(*ast.CallExpr)
	return ok && call.Fun == expr
}

func (x *extractor) spoken(b *body) ([]manifest.Speaks, error) {
	var calls []spokenCall
	ast.Inspect(b.fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		verb := x.seatOf(b, sel.X)
		if verb == nil {
			return true
		}
		if in := intentNamed(verb, sel.Sel.Name); in != nil {
			calls = append(calls, spokenCall{verb: verb, intent: in, node: call})
		}
		return true
	})
	sort.SliceStable(calls, func(i, j int) bool { return calls[i].node.Pos() < calls[j].node.Pos() })

	out := make([]manifest.Speaks, 0, len(calls))
	for _, c := range calls {
		how, err := x.consumption(b, c)
		if err != nil {
			return nil, err
		}
		out = append(out, manifest.Speaks{
			Seat:        c.verb.Seat,
			Name:        c.intent.Exchange,
			Consumption: how,
			ChainRole:   how.ChainRole(),
			Returns:     c.intent.ReturnType().Event,
			Permissions: append([]string(nil), c.verb.Permissions...),
		})
	}
	return out, nil
}

// consumption reads one spoken exchange, following the handle wherever the
// author put it and refusing wherever it cannot follow.
func (x *extractor) consumption(b *body, c spokenCall) (manifest.Consumption, error) {
	switch p := b.parents[c.node].(type) {
	case *ast.ExprStmt:
		// Spoken and walked away from: the silence IS the gate.
		return manifest.Concurrent, nil

	case *ast.SelectorExpr:
		// The handle is reached straight through, unnamed.
		if p.X != ast.Expr(c.node) || !isCallee(b, p) {
			break
		}
		switch p.Sel.Name {
		case "Value":
			return manifest.Inline, nil
		case "Received":
			// The fast beat is not consumption; the gate still stands.
			return manifest.Concurrent, nil
		}

	case *ast.CallExpr:
		// The handle is handed straight to something. Detach is the only
		// something this analyzer knows.
		if x.isDetachCall(p) && callTakes(p, c.node) {
			return manifest.Detached, nil
		}

	case *ast.AssignStmt, *ast.ValueSpec:
		name := boundName(p, c.node)
		if name == "" {
			// Bound to _ — the same as walking away from it.
			return manifest.Concurrent, nil
		}
		return x.followHandle(b, c, name)
	}
	return "", b.refuse("speaks %s and hands the handle to %s — consume it with Value(), gate on Received(), or release it with koine.Detach, in this body. A manifest the analyzer had to guess at is a declaration that could lie about the code.",
		strconv.Quote(c.intent.Exchange), x.enclosing(b, c.node))
}

// followHandle reads every use of a named handle. The name must be bound
// exactly once — a reassignment or a shadow would make the reading depend on
// which write the analyzer happened to look at, which is precisely how a
// declaration starts lying — and every use must be one the analyzer knows.
func (x *extractor) followHandle(b *body, c spokenCall, name string) (manifest.Consumption, error) {
	if len(b.writes[name]) != 1 {
		return "", b.refuse("binds %q %d times — the analyzer follows one handle per name, and a reassigned name would make %s inherit another handle's consumption. Give each handle its own name.",
			name, len(b.writes[name]), strconv.Quote(c.intent.Exchange))
	}
	consumed, detached := false, false
	for _, id := range b.readsOf(name) {
		if call, ok := b.parents[id].(*ast.CallExpr); ok && x.isDetachCall(call) && callTakes(call, id) {
			detached = true
			continue
		}
		sel, ok := b.parents[id].(*ast.SelectorExpr)
		if !ok || sel.X != ast.Expr(id) || !isCallee(b, sel) {
			return "", b.refuse("hands the handle %q to %s — consume it with Value(), gate on Received(), or release it with koine.Detach, in this body.", name, x.enclosing(b, id))
		}
		switch sel.Sel.Name {
		case "Value":
			consumed = true
		case "Received":
		default:
			return "", b.refuse("reaches %s.%s, which is not part of the Handle contract.", name, sel.Sel.Name)
		}
	}
	switch {
	case consumed && detached:
		return "", b.refuse("both consumes and detaches %q — inline and detached are different chain roles, and a station declares one or the other.", name)
	case consumed:
		return manifest.Inline, nil
	case detached:
		return manifest.Detached, nil
	}
	return manifest.Concurrent, nil
}

// isDetachCall recognises koine.Detach in both its spellings, inferred and
// explicit.
func (x *extractor) isDetachCall(call *ast.CallExpr) bool {
	fun := call.Fun
	switch idx := fun.(type) {
	case *ast.IndexExpr:
		fun = idx.X
	case *ast.IndexListExpr:
		fun = idx.X
	}
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Detach" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == x.koineLocal
}

// callTakes reports whether node is one of the call's arguments.
func callTakes(call *ast.CallExpr, node ast.Node) bool {
	for _, arg := range call.Args {
		if ast.Node(arg) == node {
			return true
		}
	}
	return false
}

// boundName is the identifier a binding statement gave to node.
func boundName(binding ast.Node, node ast.Node) string {
	switch s := binding.(type) {
	case *ast.AssignStmt:
		for i, rhs := range s.Rhs {
			if ast.Node(rhs) == node && i < len(s.Lhs) {
				if id, ok := s.Lhs[i].(*ast.Ident); ok && id.Name != "_" {
					return id.Name
				}
			}
		}
	case *ast.ValueSpec:
		for i, val := range s.Values {
			if ast.Node(val) == node && i < len(s.Names) {
				if id := s.Names[i]; id.Name != "_" {
					return id.Name
				}
			}
		}
	}
	return ""
}

// seatNeeds is the seat catalogue the door checks: every seat the station
// spoke, with what filling it requires. Sorted by seat, because a manifest
// is read by machines and diffed by people.
func seatNeeds(d *Delivery, spoken []manifest.Speaks) []manifest.SeatNeed {
	wanted := map[string]bool{}
	for _, s := range spoken {
		wanted[s.Seat] = true
	}
	var out []manifest.SeatNeed
	for _, v := range d.Verbs {
		if !wanted[v.Seat] {
			continue
		}
		out = append(out, manifest.SeatNeed{
			Seat:        v.Seat,
			Tool:        v.Tool,
			Connection:  v.Connection,
			Permissions: append([]string(nil), v.Permissions...),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seat < out[j].Seat })
	return out
}

// enclosing quotes the smallest statement or expression that holds a node,
// so a refusal shows the author their own line instead of describing it.
func (x *extractor) enclosing(b *body, node ast.Node) string {
	for n := ast.Node(node); n != nil; n = b.parents[n] {
		switch n.(type) {
		case *ast.CallExpr, *ast.ExprStmt, *ast.AssignStmt, *ast.ReturnStmt, *ast.ValueSpec:
			return strconv.Quote(x.src(n))
		}
	}
	return strconv.Quote(x.src(node))
}

// src renders a node the way the author wrote it, so a refusal can quote the
// source rather than describe it.
func (x *extractor) src(node ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, x.fset, node); err != nil {
		return fmt.Sprintf("%v", node)
	}
	return buf.String()
}

// stringLit reads a string literal, or reports that the expression was not
// one. A manifest is derived from what is written down; a computed selector
// or a computed claim is refused rather than guessed at.
func stringLit(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}
