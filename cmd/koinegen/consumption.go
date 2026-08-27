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
// handle, and the reading below is the whole analysis:
//
//   - Value() called on the handle          → inline
//   - the handle bound and koine.Detach'd   → detached
//   - anything else                         → concurrent, the default
//
// Concurrent is the default because silence must never mean "ungated". A
// station that speaks and walks away has still gated its own completion on
// the answer; only a written Detach releases it.

// spokenCall is one exchange found in a body, with everything needed to name
// its consumption.
type spokenCall struct {
	verb   *Verb
	intent *Intent
	node   *ast.CallExpr
}

func (x *extractor) spoken(station string, fn *ast.FuncDecl, d *Delivery) ([]manifest.Speaks, error) {
	var calls []spokenCall
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		seat, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return true
		}
		seatSel, ok := seat.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		for _, v := range d.Verbs {
			if v.Name != seatSel.Sel.Name {
				continue
			}
			for _, in := range v.Intents {
				if in.Name == sel.Sel.Name {
					calls = append(calls, spokenCall{verb: v, intent: in, node: call})
					return true
				}
			}
			return true
		}
		return true
	})
	sort.SliceStable(calls, func(i, j int) bool { return calls[i].node.Pos() < calls[j].node.Pos() })

	out := make([]manifest.Speaks, 0, len(calls))
	for _, c := range calls {
		how, err := x.consumption(station, fn, c)
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

func (x *extractor) consumption(station string, fn *ast.FuncDecl, c spokenCall) (manifest.Consumption, error) {
	if valueConsumed(fn, c.node) {
		return manifest.Inline, nil
	}
	name := boundTo(fn, c.node)
	if name == "" {
		return manifest.Concurrent, nil
	}
	if x.detached(fn, name) {
		return manifest.Detached, nil
	}
	return manifest.Concurrent, nil
}

// valueConsumed asks whether anything in the body calls Value() on this
// handle. Consuming the value is what makes the exchange run in the caller's
// own sequence — the same chain, the main role.
func valueConsumed(fn *ast.FuncDecl, node *ast.CallExpr) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Value" {
			return true
		}
		if sel.X == ast.Expr(node) {
			found = true
			return false
		}
		return true
	})
	return found
}

// boundTo is the identifier the handle was bound to, when it was bound at
// all. An unbound handle can never be detached — there is nothing to name in
// the Detach — so it is concurrent by construction.
func boundTo(fn *ast.FuncDecl, node *ast.CallExpr) string {
	name := ""
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if name != "" {
			return false
		}
		switch s := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range s.Rhs {
				if rhs == ast.Expr(node) && i < len(s.Lhs) {
					if ident, ok := s.Lhs[i].(*ast.Ident); ok {
						name = ident.Name
						return false
					}
				}
			}
		case *ast.ValueSpec:
			for i, v := range s.Values {
				if v == ast.Expr(node) && i < len(s.Names) {
					name = s.Names[i].Name
					return false
				}
			}
		}
		return true
	})
	return name
}

// detached asks whether the body speaks koine.Detach over this name.
// Divergence from the default gate is legal and loud: it is written, in the
// author's file, under the author's name.
func (x *extractor) detached(fn *ast.FuncDecl, name string) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		fun := call.Fun
		if idx, ok := fun.(*ast.IndexExpr); ok { // koine.Detach[T](h)
			fun = idx.X
		}
		sel, ok := fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Detach" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != x.koineLocal {
			return true
		}
		for _, arg := range call.Args {
			if ident, ok := arg.(*ast.Ident); ok && ident.Name == name {
				found = true
				return false
			}
		}
		return true
	})
	return found
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

// src renders an expression the way the author wrote it, so a refusal can
// quote the source rather than describe it.
func (x *extractor) src(expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, x.fset, expr); err != nil {
		return fmt.Sprintf("%v", expr)
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
