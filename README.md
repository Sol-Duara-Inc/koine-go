# koine-go

The Koine paradigm, rendered in Go: the SDK a station author imports. Stations declare what
they **await**, arrivals **store** until the shape of complete is satisfied, and resolving
**emits** — speech, not side effect. Everything below the line (envelopes, chains, actors,
credentials, storage-by-emission) belongs to the host engine and is structurally unreachable
from author code.

**Status:** K0 (the contracts) and K1 (codegen, the manifest, the harness) have landed.
[DESIGN.md](DESIGN.md) is the whole document — the transcribed semantics, the contract types,
the amendments from the engine build, the open-decision ledger, and the build phases that
become this repository's issues.

```
koine/          the contract types, the base stations, the handles     — stdlib only
koine/selector/ the awaits grammar                                     — stdlib only
koine/codec/    the reflection-free codec generated strata are written against
koine/manifest/ the registration manifest's shape (derived, never hand-written)
koine/testing/  the pure-Resolve harness — the author's only test surface
cmd/koinegen/   the schema registry to data strata, and the manifest extractor
```

Regenerate the committed fixtures with `go generate ./cmd/koinegen/fixtures/`; a full
regeneration must produce zero diff.

KoineDSL is public — this is its Go rendering, and the what and why of the Sol Duara demo.

**License:** [AGPL-3.0](LICENSE) while this is work in progress — changes to it are disclosed
and shared. When it lands at the CDF it is hardened for Apache 2.0 as its own pass.
