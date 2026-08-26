package koine

import "github.com/sol-duara-inc/koine-go/koine/selector"

// Identity is ownership; the commitment. It mirrors the RFC tuple. This
// package CARRIES the claim and never verifies it: group and author are
// verified by the registration door against the deployment's
// login-established directory. There is no verification hook here and no
// place to add one — the absence is the design.
type Identity struct {
	Group  string // must match IdentityGroupPattern
	Author string
	Name   string
}

// IdentityGroupPattern is the RFC grammar for Group, carried as data for
// tools (the manifest extractor, the registration door) that check shape.
// Shape-checking is grammar, not trust — trust is established at login.
const IdentityGroupPattern = `^[a-z][a-z0-9-]*$`

// String renders the tuple as group/author/name — the spelling refusals and
// records use.
func (i Identity) String() string { return i.Group + "/" + i.Author + "/" + i.Name }

// Utterance is anything a station may speak. Domain types are generated;
// authors never construct envelopes. A type becomes an Utterance by embedding
// IsUtterance — generated strata live in their own packages, and the embedded
// marker is what lets a foreign package satisfy this package's contract.
type Utterance interface{ utterance() }

// IsUtterance marks a domain type as speakable. Embed it:
//
//	type Deploy struct {
//	    koine.IsUtterance
//	    Artifact string
//	}
type IsUtterance struct{}

func (IsUtterance) utterance() {}

// Delivery is the projected facts a station is constructed with. Concrete
// deliveries are generated per-station from its Awaits; they carry your whole
// kingdom — every field your stratum comprehends — and nothing beyond it.
// A type becomes a Delivery by embedding IsDelivery.
type Delivery interface{ delivery() }

// IsDelivery marks a generated type as deliverable. Embed it.
type IsDelivery struct{}

func (IsDelivery) delivery() {}

// Yield is the speech act. Returning false stops resolution: the host has
// cancelled, and nothing after the refusal is spoken.
type Yield func(Utterance) bool

// Koine is the station contract — one plain interface, all strata, so the
// whole fleet is one uniform []Koine (ruled 2026-08-26). Typed adapters that
// hand Resolve a concrete delivery are generated; the core stays non-generic
// so every rendering of the paradigm, in any language, shares this shape.
type Koine interface {
	Identity() Identity
	Awaits() []selector.Selector     // the shapes I know what to do with
	Complete() Contract              // shape of complete; DefaultAllAwaited
	Resolve(d Delivery, yield Yield) // calculating or emitting — the data is already there
}

// Contract is when accumulated arrivals constitute "complete for my work".
// By default, all awaited shapes present; where the work needs more, the
// convergence contract is authored explicitly, never inferred.
type Contract interface{ contract() }

type allAwaited struct{}

func (allAwaited) contract() {}

// DefaultAllAwaited is the default shape of complete: every awaited shape
// has arrived.
var DefaultAllAwaited Contract = allAwaited{}
