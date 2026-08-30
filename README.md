# koine-go

The Koine paradigm, rendered in Go: the SDK a station author imports. Stations declare what
they **await**, arrivals **store** until the shape of complete is satisfied, and resolving
**emits** — speech, not side effect. Everything below the line — *forming* envelopes, chain
*continuation*, actor *minting*, auto-emission, exchange brokering, credentials — belongs to
the host engine and is structurally unreachable from author code. A station READS the chain it
stands in and the actor whose authority it carries; it can no more mint one than it can form
an envelope. (An earlier wording here named the nouns rather than the verbs, and so claimed
more than [DESIGN.md](DESIGN.md) §1 and §4 do — §4 gives `Chain()` and `Actor()` to every
stratum, read-only. The design was right; the summary was not.)

**Status:** K0 (the contracts), K1 (codegen, the manifest, the harness), K2's SDK half
(the versioned wire contract and the guest fixtures) have landed.
[DESIGN.md](DESIGN.md) is the whole document — the transcribed semantics, the contract types,
the amendments from the engine build, the open-decision ledger, and the build phases that
become this repository's issues.

```
koine/          the contract types, the base stations, the handles
koine/selector/ the awaits grammar
koine/codec/    the reflection-free codec generated strata are written against
koine/manifest/ the registration manifest's shape (derived, never hand-written)
koine/testing/  the pure-Resolve harness — the author's only test surface
koine/wire/     the versioned guest contract — the one coupling with the engine
cmd/koinegen/   the schema registry to data strata, and the manifest extractor
fixtures/guest/ the conformance guests the engine loads, built with TinyGo
conformance/    a separate module: the test that crosses into the real engine
```

The engine's `pkg/koinehost` is **normative** — it defines the wire and this SDK conforms
(ruled 2026-08-28). `conformance/` is what holds that true: it builds the fixture guests and
hands them to the real loader, so a change either side makes that the other cannot read fails
by name instead of shipping. It is a module of its own precisely so the SDK's own
zero-dependency law is untouched:

```
cd conformance && go test ./...    # needs a conduit-go checkout beside this one
```

Every package in the SDK module imports the standard library and this module only — no
`go.sum`, no third-party anything — and an architecture test fails the build if that ever stops
being true. (`conformance/` is a separate module and does depend on the engine; Go excludes it
from the SDK module's package patterns entirely.)

Regenerate the committed fixtures with `go generate ./cmd/koinegen/fixtures/` and
`go generate ./fixtures/guest/...`; a full regeneration must produce zero diff. Build a guest
the way the engine does:

```
tinygo build -o steward.wasm -target wasm-unknown ./fixtures/guest/steward
```

KoineDSL is public — this is its Go rendering, and the what and why of the Sol Duara demo.

**License:** [AGPL-3.0](LICENSE) while this is work in progress — changes to it are disclosed
and shared. When it lands at the CDF it is hardened for Apache 2.0 as its own pass.
