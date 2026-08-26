# koine-go — the Go rendering of the Koine paradigm

**Sol Duara internal. This repository is private; Draft 1's stealth note ("KoineDSL is never
named externally") stands until the owner rules on visibility.**

**Status:** Draft 2 of the implementation design — 2026-08-26. Supersedes Draft 1 (2026-07-30,
`conduit-go/docs/koinedsl_go_implementation_design.md`; its full text is preserved in that
repo's history). This document is written to the ticket standard: **a stranger-agent with no
access to Sol Duara's other repositories, chat history, or memory must be able to execute each
build phase from this document alone.** Every passage the build depends on is transcribed here
in full. Nothing contested is resolved silently in code — §"Escalations" is the ledger of open
decisions, and each names what it blocks.

**The standing project ruling (owner, 2026-08-26, verbatim):**

> That is a separate standalone micro-project that can be written now. A separate project means
> a separate set of GH Issues. The document that lands in github.com/sol-duara-inc/koine-go
> should be written in the deterministic fashion that we have been using for GH Issues.

This ratifies Draft 1's escalation E-B (the module split) in stronger form: not a separate
module in a shared tree but a separate repository, tracker, and release cadence. Vendors and
customers import this SDK; they never import the engine.

---

## 1. Position

The Koine paradigm is an event language: stations declare what they **await**, arrivals
**store** until the shape of complete is satisfied, and resolving **emits** — speech, not side
effect. This repository renders the *author's side* of that paradigm in Go. Everything below
the line — forming envelopes, chain continuation, actor minting, auto-emission, exchange
brokering, credentials — is host-side, lives in the engine (`conduit-go`, package
`pkg/koinehost`, built separately), and is structurally unreachable from author code.

The paradigm ruling that governs the language choice (owner, 2026-08-24, answering the
host-language question): **Koine is a paradigm, not a language.** Go is the first rendering;
Java and Rust renderings are expected later. Nothing in this SDK may depend on a Go-only
semantic — the contract types and the wire protocol are the paradigm; Go idiom is the clothing.

Stations run as **untrusted WebAssembly guests** in the engine's wazero sandbox (no WASI, no
filesystem, no sockets, event JSON only, mediated egress). That constraint is not a limitation
to design around — it *is* the architecture: the quarantine modality realized. The author's Go
compiles to a guest; the host/guest boundary is exactly the paradigm's "names above, common
interface below."

Go strengthens the model at every joint: projection becomes the type system (boundaries checked
at compile time), divergence-by-declaration becomes method shadowing (Go has no silent
override), seats become interfaces (unfilled seats caught at registration), and plane
discipline becomes reachability (the observer type simply lacks the execution verbs).

## 2. The semantics this SDK renders — transcribed, load-bearing

The engine-agnostic definition is KoineDSL Draft 2 (2026-07). Its load-bearing passages are
transcribed here verbatim so no external document is load-bearing; a later ruling overturns any
of them by juxtaposition — new words beside old — never by a stale pointer.

### The three verbs

> 1. **awaits** — The author is the person expecting the items. Before any work, the user
>    verifies the shape of the expected item: the class-level declaration of the shapes this
>    station knows what to do with. Only the party who declared comprehension receives; there
>    are no bystanders and no addressees. Expectations form the wiring — the expected graph
>    *is* the routing table, written before runtime.
> 2. **stores** — Arrivals accumulate until the **shape of complete** is satisfied. The author
>    knows in advance the shape of complete for the work they are about to do: by default, all
>    awaited shapes present; where the work needs more — temporal windows, relational
>    constraints among arrivals — the convergence contract is authored explicitly, never
>    inferred.
> 3. **emits** — The work runs, and the station speaks — as language, not as side effect.
>    Inside `resolve()` there are only two verbs: **calculating or emitting. The data is
>    already there** — nothing is fetched, waited on, or re-verified; work is deriving from
>    what was delivered and yielding what comes next.

### What emission is

> `resolve()` **yields** its utterances: the author yields the named object of their domain,
> and **Conduit at the base** forms it — envelope, chain continuation, actor, identity,
> provenance — into the event that **fills the caller's await**. The invocation was an
> utterance expecting completion; the yield is what completes it, the way a return value
> completes a call. **Emitting the event is the storage action.** There is no database beside
> the relay and no separate send: to speak is to write is to deliver.

> A `resolve()` is a function from delivery to utterances — a Koine is testable by calling it
> and collecting the yields, no platform required.

### One calling convention

> Both faces are pre-written. The author declares what they await *and* pre-writes the
> connectors and the structure of what they intend to send. A Koine is therefore a **typed
> transform with both faces declared**, and the entire topology — who feeds whom, where shapes
> mismatch, where an expectation has no speaker — is statically derivable before anything runs.

> Calling a Koine is emitting an utterance its awaits match. There is one calling convention in
> the entire system, and it is the fabric — and the claim is universal: even a Koine's API
> calls, internal or external, are utterances whose responses arrive as expected events in a
> handler's clothing. **No second channel exists anywhere.**

A call out to a tool is an utterance; the tool's response arrives as an expected event that
fills an await. There is no separate return path to design.

### What a plugin ships

> A tool plugin ships **twin hierarchies**, both carrying interpretation by lineage: **data
> classes** (the projection surface — community floor plus vendor keys) and **behavior
> classes** (the capability surface — Koine bases **with the tool's full client injected**).

> As standard practice, every plugin ships its **reconciliation** for every event type it
> declares: the absence station (interrogate the tool when its expected utterance goes missing;
> disposition the chain with evidence, or extend within engine policy, or escalate) and the
> gap-fill enrichment (query the tool for context the event didn't carry; enrich within the
> vendor namespace).

"With the tool's full client injected" is the address ruling (owner, 2026-08-21): the plugin
does not carry, discover, or declare where its tool lives — the operator owns the address and
the credential; the deployment binds the client and hands it in. A plugin declares the tool
*name* it serves and the *shape* of connection it needs, never the location.

### Inheritance and the unfilled seat

> The parent runs by default. The parent is not an imposition to escape; it is the codified
> agreement itself, and running it is honoring it.

> Opting out is legal and loud. You have to tell it not to run. A silent skip — occupying the
> slot while quietly not executing the agreed behavior — would be the architecture lying to
> itself. You may diverge; you may not diverge secretly.

> Seats fill at any layer; the lookup walks the lineage. `NotImplementedError` is the base
> control mechanism: the floor declares the intent surfaces (the seats); any layer may fill
> one; whoever fills a seat answers for everyone above, and an unfilled seat announces itself
> by name.

> Downward blindness is respect, by design. Vendors are not aware of their customers' extended
> use of the tools they have modified. The confidentiality is not a policy that must be
> audited; it costs nothing to keep because it cannot be broken.

### The record, and history

> Because emitting is the storage action, the record is the accumulated utterances of the
> fleet. Every stored record carries, by construction rather than curation: who spoke it,
> whether it was awaited, the speaker's standing when it spoke, and the strata of who can ever
> hear it. Provenance, expectation, and trust are **columns, not annotations**.

> History is just another awaited shape. Last-good-deploy exists because every past emission
> was already a write; a station that awaits history is resolved from the record of what was
> spoken, projected like everything else. No query surface, no second convention.

The engine-side ruling that grounds this (owner, 2026-08-24): **every read of the past is
served from the record** — the database or the archive — never from resident memory. A History
fulfiller answers from the record; the SDK never caches.

### Closures — ratified 2026-08-23 (owner, verbatim)

> Absolutely, correct. The closure is DONE is the correct posture. That has always been the
> design. That is why started/finish can not traverse CDrus Expressions. The whole system is
> really just closures that share a type and the event object that make them up. Our closure
> concept mimics: {}.

A started/finished pair brackets a coordinate interval; when `finished` resolves, everything
sandwiched in the declared interval has resolved behind it, and the closure — every event in
the bracket the reader has access to, projected to their comprehension, composed into ONE
object named by the subject — is available. Closures are constructed by the engine, not this
SDK; they will surface here as delivery types for finished anchors. **Not in phases K0–K4**;
recorded so no SDK shape forecloses them.

### The resolved idiom — ratified 2026-07-20

Authors await the **completed thought**: a terminal set passes regardless of outcome, and
outcome-filtering at the declaration is an anti-pattern. The selector grammar spells this as
`resolved()` (either outcome) and `absent()` (the expectation that did not arrive); the
station's body branches on outcome, because code is written against BOTH branches of the
expected future — "when this succeeds, do this thing and should this fail, do a different
thing. But that is no different than how any code works" (owner, 2026-08-23).

## 3. Module layout

```
github.com/sol-duara-inc/koine-go        # THIS repository — the SDK authors import
  koine/          — core contract types, Base stations, registration; ZERO dependencies
  koine/selector/ — awaits grammar (selectors, anchors, resolved(), absent())
  koine/testing/  — the pure-Resolve test harness (the author's ONLY test surface)
  cmd/koinegen/   — codegen: schema registry → data strata; manifest extraction

github.com/solduara/conduit-go           # the engine (existing, separate tracker)
  pkg/koinehost/  — host-side runtime: guest loader, host functions, forming,
                    auto-emission, exchange broker, registration validator
```

The coupling between the two repositories is exactly one thing: **the wire and host-function
contract**, versioned in this repository (`koine/wire`) and conformance-gated on both sides
(K4). The SDK never imports the engine; the engine vendors nothing from the SDK but the wire
contract.

## 4. Core contract types (`koine`)

```go
// Identity — ownership; the commitment. The RFC tuple. The SDK CARRIES this
// claim; it never verifies it. Group and author are verified by the
// registration door against the deployment's login-established directory —
// there is no verification hook here and no place to add one.
type Identity struct {
    Group  string // ^[a-z][a-z0-9-]*$
    Author string
    Name   string
}

// Utterance — anything a station may speak. Implemented by generated domain
// types; authors never construct envelopes.
type Utterance interface{ utterance() }

// Delivery — the projected facts a station is constructed with. Concrete
// deliveries are generated per-station from its Awaits.
type Delivery interface{ delivery() }

// Yield — the speech act. Returning false stops resolution (host cancelled).
type Yield func(Utterance) bool

// Koine — the station contract. One interface, all strata.
type Koine interface {
    Identity() Identity
    Awaits() []selector.Selector      // the shapes I know what to do with
    Complete() Contract               // shape of complete; DefaultAllAwaited
    Resolve(d Delivery, yield Yield)  // calculating or emitting — the data is already there
}

// Contract — when accumulated arrivals constitute "complete for my work".
type Contract interface{ contract() }
var DefaultAllAwaited Contract = allAwaited{}
```

`Resolve` takes the range-over-func iterator shape (Go ≥1.23): the author yields named domain
objects; the host forms each into the enveloped event — provenance, chainId continuation,
actor — and **emitting is the storage action**, host-side, unreachable and unsuppressable from
the guest.

### The base stations — plane discipline as reachability

```go
// Base — embed in every station. Carries the standard parts and the
// host-mediated context. Construction is delivery: the host builds the
// station; authors never call NewBase.
type Base struct{ ctx hostContext }

func (b *Base) Chain() ChainRef        // where I stand in the flow (read-only, all strata)
func (b *Base) Actor() ActorRef        // whose authority I carry (sub/act), minted by the host
func (b *Base) Project() ProjectContext // the pinned interface that keeps a body tool-free
```

Two stratum bases, differing **only in reachable verbs** — never in permission checks:

```go
// ObserverBase — the y stratum. Hears like everyone; holds no chain verbs.
type ObserverBase struct{ Base }

// ExecutionBase — the m stratum (plugin authors, workflow owners).
type ExecutionBase struct{ Base }
func (e *ExecutionBase) ResolveChain(outcome Outcome, evidence ...Evidence) // evidence required, host-enforced non-empty
func (e *ExecutionBase) ExtendChain(within time.Duration)                   // engine-policy-capped re-arm
```

An observer station containing `ResolveChain` does not compile. No configuration can create the
call; the verb does not exist from that position. This is the SDK's standing principle, learned
the hard way on the engine side and now a design law here: **where a path must not exist,
remove the affordance — never build the path and guard it.** No debug hooks, no test-only
bypasses, no "bench mode" in this SDK, ever.

## 5. Data strata: generated, embedded, projected

`koinegen` consumes the schema registry (reverse-DNS namespace format and inheritance) and
emits one struct per stratum, embedding downward to the floor:

```go
// floor (generated from CDEvents schemas)
type BuildFinished struct {
    Subject    SubjectRef
    Outcome    Outcome
    ArtifactID ArtifactRef
    // ... community keys only
}
func (BuildFinished) utterance() {}

// vendor stratum (generated from an io.jenkins registration)
type JenkinsBuildFinished struct {
    BuildFinished              // the agreement, embedded
    ExecutorNode string
    QueueWaitMS  int
}

// customer stratum (generated from com.example.payments-engineering)
type PaymentsBuildFinished struct {
    JenkinsBuildFinished
    // private keys
}
```

**Projection is the type system.** A floor station's Delivery is typed `BuildFinished`; vendor
fields do not exist in its type — the boundary is a compile error, not a runtime filter. On the
wire, the host projects before the guest ever sees bytes: the guest is handed JSON already cut
to its lineage, and unmarshals into its stratum type. Two walls, one semantic. Seeding
constructors are generated only at the owning domain's stratum (objects are seeded whole at the
deepest extension — ruled 2026-07-21).

**Divergence by declaration, enforced by Go itself:** embedding promotes the parent's standard
parts; running them is the default because *not* running them requires writing a shadowing
method — in your file, under your name, in the blame. The agreement-by-default rule costs zero
SDK code.

## 6. Verbs, exchanges, and handles

Author-facing verbs are generated methods on delivery types, curried with station context at
construction:

```go
last := d.History().Last(koine.Success)   // an intent, spoken
```

Under the hood every verb call is a **spoken exchange** through one host function: the host
emits the request utterance, the fulfiller's `work.received` is the acknowledgment beat,
`work.finished` carries the response — the work-plane protocol verbatim, one calling
convention, no second channel. The SDK exposes it through **handles**:

```go
type Handle[T any] interface {
    Received() Ack        // gate on "someone comprehended this" (fast beat)
    Value() (T, error)    // gate on completion (materializes; blocks the guest;
                          //   err is the typed outcome variant, never transport)
}
```

**Branch control by consumption (ratified 2026-07-30) compiles from usage:**

- `h.Value()` consumed → **inline**: the exchange runs in the caller's own sequence — the same
  chain.
- Handle spoken, never consumed → **concurrent (default)**: the host gates this station's
  completion on the exchange's resolution — a **blocking chain**; nothing is silently ungated.
- `koine.Detach(h)` → **detached, by declaration**: released from the gate, in code, under the
  author's name — a **detached chain**, the same word as the grammar's keyword.

The three consumption patterns map one-to-one onto the engine's three chain roles
(`main` / `blocking` / `detached` — the only roles its stored expected graph admits), which is
what makes the next section's manifest a topology, not a listing.

## 7. Registration: the manifest, derived from code

`koinegen manifest` extracts, per station, at build time: identity; awaits (selectors,
content-hash pinned); declared emit types; every verb/seat spoken with its consumption pattern;
namespace lineage (from embedding); and, for plugin stations, the connection shape and the
external permission requirements per seat. **The manifest is derived from the code — never
hand-written** — so declaration and behavior cannot drift; a conformance test in this
repository pins that the extractor's output matches what the station's compiled body actually
speaks.

The manifest is exported from the compiled guest under the export name `manifest`, read once by
the host at load, bounded, and validated **before any guest executes** — the same door, the
same reader, and the same JSON shape family the engine's tool plugins already use (claim, tool,
connection shape, declared events each with its reconciliation entry), extended with the Koine
fields (awaits, emits, consumption topology, stratum). One loader serves both kinds of guest.

Registration judges in order and **refuses by name, storing nothing on refusal**:

1. Identity: group and author must be valid in the deployment's directory — verified against
   what login already established, never re-proven at this door.
2. Selectors: resolved against the expression store the registration cites, content-hash
   pinned; a selector path is type-checked against the schema versions that pin names.
3. Seats: every seat the station speaks must be declared in the shared catalogue; an unfilled
   seat — no fulfiller in the deployment's claim-resolution index — is refused as
   `NotImplemented`, **naming the seat**: the honest terminus before runtime. (At read time the
   same facts answer in the deployment's three-state capability vocabulary — `ready`,
   `connect-your-account`, `nobody-connected` — one truth, two readers.)
4. Plane: vacuous by construction (§4), re-verified on the manifest for defense in depth
   against hand-built manifests.
5. Topology: the consumption analysis yields the station's expected exchange graph — inline /
   blocking / detached per exchange — so the engine can mint proleptic coordinates for the
   station's work exactly as it does for authored workflows, and the standing budget calculus
   (a blocked parent's derived budget is the max over sibling chains, the sum within each;
   detached chains excluded) applies with **no new arithmetic**.

## 8. The host below the line (engine-side, for context only)

Host functions exposed to guests — the *entire* guest-visible surface: `deliver` (host→guest:
projected JSON + construction context), `yield` (guest→host: domain object; host forms the
envelope and stores-by-emitting), `exchange` (verb calls; the host brokers the work-plane
beats, mints the sub/act token, routes via the deployment's connectors), `ack_poll` /
`value_poll` (handle beats). Auto-emission of `work.received` / `work.started` /
`work.finished(outcome)` is host-side around guest lifecycle — the guest has no path to
suppress it because the guest has no emit path at all except `yield`. Exceptions/traps in the
guest become `work.finished{outcome: failure}` with the trap attributed. This package
(`conduit-go/pkg/koinehost`) is built in the engine's tracker, against the wire contract
versioned here.

## 9. Testing: the harness is the author's only street

`Resolve` is a pure function from delivery to utterances. `koine/testing` ships the complete
authoring test surface:

```go
out := koinetest.Run(DeploymentSteward{},
        koinetest.Deliver(deployment.ResolvedDelivery{Outcome: koine.Failure /* ... */}),
        koinetest.Exchange("history.last", lastGoodFixture))   // scripted expected responses
// assert on out.Utterances, out.Exchanges, out.Consumption (inline/concurrent/detached)
```

No engine, no sandbox, no network, no daemon — and none of those are offered. An author who
wants to test against a live engine is on the deployment's side of the line, with the
deployment's own tools; this SDK does not carry that door. Because a History fulfiller is
record-backed and the SDK never caches, a station is fully deterministic given its delivery
plus scripted exchanges — the harness is not a mock of the semantics, it IS the semantics minus
the transport.

The proof this design leans on, run as an experiment during the engine build: one station body,
byte-identical, resolved the same failed-deployment delivery against three different
deployments whose history truth lived in three different places (the engine's own record, a
deployment tool, a bare artifact registry). Nothing in the author's code named a tool, and
nothing in it changed. The host answered the intent; the locution was the same everywhere. That
is the usability claim of this whole repository, and K4 pins it as a conformance scenario.

## 10. Worked example (compiles against this design)

```go
type DeploymentSteward struct{ koine.ObserverBase }

func (DeploymentSteward) Identity() koine.Identity {
    return koine.Identity{Group: "payment-engineering", Author: "mchen", Name: "deployment-steward"}
}
func (DeploymentSteward) Awaits() []selector.Selector {
    return selector.List(deployment.Resolved())     // the completed thought — either outcome
}
func (DeploymentSteward) Complete() koine.Contract { return koine.DefaultAllAwaited }

func (s DeploymentSteward) Resolve(d koine.Delivery, yield koine.Yield) {
    dep := d.(deployment.ResolvedDelivery)          // generated; typed to the station's lineage
    if dep.Outcome == koine.Success {
        yield(DeploymentRecorded{Artifact: dep.ArtifactID})
        return
    }
    lastGood, err := dep.History().Last(koine.Success).Value()  // inline: consumed
    if err != nil { /* typed variant of the expected response; branch, don't defend */ }
    yield(Deploy{Artifact: lastGood.ArtifactID, Target: dep.Environment})
}
```

## 11. Amendments from the engine build — 2026-08-26

Draft 1 predates most of the engine. Each amendment below records what changed from Draft 1,
why (what the build proved), and what it buys. Nothing else in Draft 1 was altered.

- **A1 — Module path.** `github.com/solduara/koine-go` → `github.com/sol-duara-inc/koine-go`:
  the owner named the address, and the org's standalone-library precedent
  (`sol-duara-inc/conduit-layers-go`) uses the org path. *Buys:* imports match the repository
  people actually find.
- **A2 — One manifest door.** Draft 1 had `koinegen manifest` producing its own artifact. The
  engine has since shipped a real manifest mechanism for wasm guests — a `manifest` export read
  once at load, size-bounded, held beside the guest's capabilities, validated at boot with
  refusal by name. This design now targets THAT door: the station manifest is the same export
  name, the same read discipline, and the same shape family (claim / tool / connection shape /
  declared events with reconciliation), extended with the Koine fields. *Buys:* one loader and
  one boot validator for tool plugins and stations alike; no second manifest reader to drift.
- **A3 — Declarations derived from code, pinned by conformance test.** The engine's performers
  declare the seats their switch already serves, and a conformance test pins declaration to
  behavior. Same law here: `koinegen` derives the manifest from the compiled station — awaits,
  emits, seats, consumption — and a test asserts extractor-output equals behavior. Hand-written
  manifests are refused work, not a supported path. *Buys:* the declaration can never lie about
  the code.
- **A4 — Consumption patterns ARE chain roles.** Draft 1 said the topology "stays derivable";
  the engine build makes the mapping exact: inline = the caller's own chain, spoken-unconsumed
  = a blocking chain, `Detach` = a detached chain — the only three roles the engine's stored
  expected graph admits. Registration therefore yields a proleptic coordinate topology for a
  station's exchanges, and the engine's existing budget calculus (max over sibling chains, sum
  within each; detached excluded) applies unchanged. *Buys:* stations get minted coordinates
  and TTL arithmetic for free; no new calculus, no new tables.
- **A5 — Seat vocabulary unified.** Draft 1 said "capability"; the ruled word is **seat**, and
  the deployment's read surface answers seat states in a ruled three-state vocabulary (`ready`
  / `connect-your-account` / `nobody-connected`). Registration refusals and read-time dark
  states now speak the same names. *Buys:* an author reads the same word at compile,
  registration, and console.
- **A6 — Zero-dependency core, architecture-pinned.** The org's layers library proved the
  discipline: `koine/` and `koine/selector/` import the standard library only — no engine, no
  wasm runtime, no third-party anything — and an architecture test enforces it (a guard that
  fails the build on a foreign import, the way the engine pins its attribution boundary).
  Codegen emits reflection-free marshal/unmarshal so the guest target (E-A) stays open.
  *Buys:* the SDK embeds anywhere Go compiles, TinyGo included; the dependency answer to a
  security review is one sentence.
- **A7 — The harness is the only street.** Learned in the engine, ruled by the owner: shipping
  test affordances against a live surface keeps everyone wanting to use them. `koine/testing`
  is complete and engine-free (§9), and the SDK offers no path to a running engine at all.
  *Buys:* authors cannot form the habit this rule exists to prevent.
- **A8 — Identity carried, never verified.** Draft 1 was silent on who proves `Identity`.
  Ruled since, engine-side: group and author are verified at login/registration by the door
  that already holds the directory truth — never re-proven downstream, and never by the SDK.
  The SDK carries the claim and contains no verification hook. *Buys:* no station can be
  written that second-guesses the deployment's identity truth, and no reviewer can mistake the
  absence of a check for a gap — the absence is the design.
- **A9 — Refusal ladder, storing nothing.** The engine's authoring doors ruled the posture:
  everything must validate or be refused, judged in order, refused by name, nothing stored on
  refusal. §7's registration sequence adopts it verbatim. *Buys:* a failed registration leaves
  no partial station anywhere, and every refusal is actionable by name.

## 12. Escalations — the open-decision ledger

Nothing here is resolved in code. Each row names its recommendation and exactly what it blocks.
**E-B is ratified** (the owner's 2026-08-26 ruling, quoted at the top); the other four await
one word each.

| id | decision | recommendation | blocks |
|---|---|---|---|
| E-A | Guest toolchain: standard Go's wasm output is large and goroutine-heavy for the sandbox; TinyGo is lean but restricts reflection (fine — codegen is reflection-free by A6) | TinyGo as the supported guest target; standard Go permitted for development | K2 (the first guest actually loaded) |
| E-B | Module split | **RATIFIED 2026-08-26** — separate repository, tracker, and issues | — |
| E-C | `Value()` blocks the guest while the host brokers the exchange (single-threaded guest) | correct and simple for v1; async continuation styles deferred | K3 (handle semantics frozen) |
| E-D | Delivery type assertion in author code vs a generic `Koine[D Delivery]` | core stays non-generic; generated typed wrappers keep the assertion out of author code | K0 (the `Koine` interface signature) |
| E-E | Observer SDK surface: same module, or a trimmed observer-only module for wide distribution | same module for v1 | K0 (package layout) |

## 13. Build phases → the issue set

Each phase below is written to become one GitHub issue in this repository, verbatim, when the
owner ratifies this document. Every phase carries its own done-conditions and acceptance
commands; a phase is not done until its commands are green **with the named tests visibly run**
(`go test -v -run <pattern>` must print `=== RUN` for each named test — an `ok` with no test
names means the pattern matched nothing and proves nothing).

### K0 — the contracts (blocked by: E-D, E-E)

**Objective:** `koine/`, `koine/selector/` compile and are exhaustively unit-tested; no host,
no codegen, no wasm.

**Done when:** the §4 types exist exactly as written (modulo E-D's ratified answer); the
selector grammar covers typed selectors, anchors, `Resolved()` (either outcome — the resolved
idiom), `Absent()`; `Contract` with `DefaultAllAwaited`; `Handle[T]` with `Received()`/
`Value()`; `Detach`; the zero-dependency architecture test passes and FAILS when a third-party
import is added (prove once by adding one, watching it fail, removing it).

**Acceptance:** `go build ./... && go vet ./...` clean; `gofmt -l .` empty;
`go test -race -count=1 -v ./koine/... -run 'TestSelector_|TestContract_|TestHandle_|TestArch_StdlibOnly'`
green with every name printed.

### K1 — codegen (blocked by: K0)

**Objective:** `koinegen` over a fixture schema registry (committed under
`cmd/koinegen/testdata/`) generates a floor stratum, one vendor stratum, one customer stratum —
embedding, projection-by-type, seeding constructors at the owning stratum only,
reflection-free marshal/unmarshal — plus `koinegen manifest` extracting the §7 manifest from a
compiled fixture station.

**Done when:** generated goldens are committed and byte-stable (regeneration produces zero
diff); a floor station referencing a vendor field fails to compile (pinned by a
compile-refusal test fixture); the extracted manifest for the §10 worked example names its
identity, one await, its emits, and the `history.last` exchange as **inline**.

**Acceptance:** `go test -race -count=1 -v ./cmd/koinegen/... -run 'TestGenerate_|TestManifest_'`
green with names printed; `git diff --exit-code` after a full regeneration.

### K2 — host protocol (engine-side; blocked by: K1, E-A)

**Objective:** `conduit-go/pkg/koinehost` — the guest loader and the five host functions
(`deliver`, `yield`, `exchange`, `ack_poll`, `value_poll`), auto-emission around guest
lifecycle, manifest read at load through the engine's existing manifest door. One end-to-end
station (the §10 steward) compiled to wasm and running in the sandbox.

**Placement:** the code lands in the engine's repository under its tracker; THIS repository's
K2 issue covers the SDK side — `koine/wire` (the versioned wire contract) and the guest-side
conformance fixtures the engine loads. The two issues reference each other by number and ship
in the same release.

**Done when:** the steward guest loads, receives a projected delivery, yields, and the host
forms and stores the emission; a guest trap surfaces as `work.finished{outcome: failure}` with
the trap attributed; the guest demonstrably has no emit path but `yield` (a fixture guest that
tries a second export path is refused at load, by name).

### K3 — exchanges and branching (blocked by: K2, E-C)

**Objective:** handle beats end to end; consumption analysis enforced (inline / blocking /
detached land as the three chain roles); `Detach` released from the completion gate;
per-seat permission requirements flow from the manifest into the deployment's outbound test.

**Done when:** the three consumption patterns each produce their chain role in the engine's
stored expected graph for a fixture station, and the blocking pattern gates completion (a
never-answered exchange breaches on the engine's standing budget calculus, not a new one).

### K4 — conformance (blocked by: K3)

**Objective:** the Koine conformance suite: paradigm semantics asserted against the Go
rendering — the resolved-idiom totality check, the projection walls, agreement-by-default
(shadowing is the only override), manifest-matches-behavior (A3), and the three-hosts scenario
from §9 (one station body, three history fulfillers, byte-identical author code).

**Done when:** the suite runs green from this repository against a pinned engine version, and
the engine's CI runs the same suite against its head — the wire contract is the only coupling,
proven by the gate on both sides.

---

*Draft 2, for ratification. On the owner's word: E-A/C/D/E resolve (one line each), this
document merges, and K0 files as the first issue. Findings that contradict this design are
surfaced, not silently changed.*
