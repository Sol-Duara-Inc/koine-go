# koine-go

The Koine paradigm, rendered in Go: the SDK a station author imports. Stations declare what
they **await**, arrivals **store** until the shape of complete is satisfied, and resolving
**emits** — speech, not side effect. Everything below the line (envelopes, chains, actors,
credentials, storage-by-emission) belongs to the host engine and is structurally unreachable
from author code.

**Status:** design ratification. [DESIGN.md](DESIGN.md) is the whole document — the
transcribed semantics, the contract types, the amendments from the engine build, the
open-decision ledger, and the build phases that become this repository's issues.

KoineDSL is public — this is its Go rendering, and the what and why of the Sol Duara demo.

**License:** [AGPL-3.0](LICENSE) while this is work in progress — changes to it are disclosed
and shared. When it lands at the CDF it is hardened for Apache 2.0 as its own pass.
