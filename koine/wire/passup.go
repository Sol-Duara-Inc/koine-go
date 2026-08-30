package wire

import (
	"strconv"

	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/codec"
)

// The guest half of the pass-up (koine-go#5), riding the K2 exchange ABI.
//
// # PROPOSED SPELLINGS — read this before believing any of it
//
// The host half is conduit-go#210 (ChainBroker). AT THE TIME OF WRITING IT
// DOES NOT EXIST: there is no ChainBroker, no pass-up store, and no reserved
// type in conduit-go. So unlike everything else in this package, what follows
// is NOT transcribed from a merged host — it is this SDK's PROPOSAL, and it
// is gathered in one file so that agreeing it with #210 changes one file.
//
// K2 shipped two halves of one wire written against two readings of it, and
// the postmortem was that both suites passing is exactly what let them drift.
// The lesson is not "read harder"; it is "do not call a proposal a
// conformance". Each item below says what is agreed, by whom, and what is
// still open:
//
//  1. TYPE STRING — "koine.passup". BOTH tickets name this string, and #210
//     is being updated to adopt these spellings as its own done-conditions,
//     so "as agreed with the host ticket" becomes real rather than
//     aspirational. Asserted in conduit-go's code not yet. TypePassUp below
//     is the one place this SDK spells it — with a mirror in cmd/koinegen,
//     which cannot import this package, pinned to it by a test.
//  2. THE CARRIER — the offered object rides ExchangeRequest.Intent, the only
//     field of that shape. OPEN: #210 does not say which field. Worse, the
//     law says the door already minted one object per lineage layer and
//     "nothing is re-spoken, nothing re-verified", which reads as the host
//     already HOLDING the parent's object — in which case the guest need ship
//     no bytes at all. RESOLVED by the owner 2026-08-30: both texts are true
//     on DIFFERENT PATHS. An explicit pass — the verbs, or a Pre transform —
//     ships the object here, because the transform is the point. The
//     zero-code default pass is the HOST's, served from the object the door
//     already minted, with no guest bytes at all. "Nothing is re-spoken"
//     governs the default; "Pre may transform" governs the explicit. This
//     SDK implements exactly that: a station that writes nothing emits
//     nothing, and every byte below belongs to a station that wrote something.
//  3. THE OBJECT'S SHAPE — the domain object with its type key spliced flat,
//     exactly as YieldFrame does it, because the host reads a yield's type the
//     same way and one convention beats two. OPEN, same as (2).
//  4. WITHHOLD — a second reserved type, "koine.withhold". OPEN, and neither
//     ticket says: koine-go#5 defers to "as agreed with the host ticket" and
//     conduit-go#210 has no Withhold done-condition at all. A second type is
//     proposed over a manifest flag because the manifest is read ONCE at load
//     and a withhold is a per-delivery decision, and over a field on the
//     pass-up exchange because a withhold is precisely the case where no
//     pass-up is spoken.
//  5. THE CONCLUSION FRAME — the parent's outcome arrives as an ordinary
//     ExchangeResponse. A parent that concluded plainly answers with no value,
//     and for a PASS-UP that is the ordinary case, not a stoppage — see
//     awaitPass. OPEN: #210 says only "answers the parent's outcome".
//  6. THE UNFILLED PARENT — a namespace declared and not loaded answers
//     determinately, in the ErrNotImplemented family. conduit-go#210 now
//     pins Status 501, an Error beginning with koine.ErrNotImplemented's own
//     text, and Breached false. Recognised below by that exact prefix.

// TypePassUp is the reserved exchange type a pass-up rides. Both tickets name
// this string; nothing in conduit-go asserts it yet. The conformance module
// pins it so that the day the host names its own constant, a disagreement is
// a failing test rather than a silent misroute.
const TypePassUp = "koine.passup"

// TypeWithhold is the reserved exchange type a withheld pass rides. PROPOSED
// — see item 4 above.
const TypeWithhold = "koine.withhold"

// UnfilledPrefix is how an unfilled parent is recognised: the exact text of
// the engine's own koine.ErrNotImplemented (conduit-go pkg/koine/resolve.go).
// conduit-go#210 pins that the host answers a declared-but-not-loaded parent
// with Status 501, an Error beginning with this text, and Breached false.
//
// It is matched by prefix rather than compared whole because the host's
// sentence names the namespace after it, which is the part a person needs.
const UnfilledPrefix = "koine: function not implemented"

// Unfilled is a parent namespace that a workflow declared and no controller
// filled. It is determinate and it is NOT a stoppage below the line: nothing
// malfunctioned, the seat is simply empty, and the same child starts working
// the day the parent installs with nothing rewritten. A body may branch on it
// exactly as it branches on any other conclusion.
type Unfilled struct {
	// Why is the host's own sentence, which names the namespace.
	Why string
}

func (u *Unfilled) Error() string { return u.Why }

// lineage is the guest's implementation of koine.Lineage. It speaks the same
// three host functions everything else here speaks; a pass-up is an exchange,
// and the parent chain is the first real broker.
type lineage struct{ g *Guest }

// PassUp hands the parent its object now.
func (l lineage) PassUp(u koine.Utterance) koine.Passage {
	if l.g.fault != "" {
		return 0
	}
	// One passage per delivery, and Withhold beats anything not yet said.
	// A suppressed pass is a no-op; a second pass traps, because two
	// passes can carry two different payloads.
	if !l.g.passing.Offer() {
		return 0
	}
	handle := l.g.host.Exchange(l.offer(TypePassUp, u))
	if handle == 0 {
		l.g.faultBelowTheLine("the host would not take a pass-up to this station's parent")
		return 0
	}
	return koine.Passage(handle)
}

// Withhold suppresses the default pass. It speaks, because the DEFAULT is the
// host's: a host that heard nothing would pass up at step end, so silence
// cannot be how a guest suppresses it.
func (l lineage) Withhold(u koine.Utterance) {
	if l.g.fault != "" {
		return
	}
	// Withholding after the parent already has it traps: nothing here can
	// unsay what was said.
	if !l.g.passing.Suppress() {
		return
	}
	if handle := l.g.host.Exchange(l.offer(TypeWithhold, u)); handle == 0 {
		l.g.faultBelowTheLine("the host would not take a withhold from this station")
	}
}

// AwaitPass waits until the parent's step concludes.
func (l lineage) AwaitPass(p koine.Passage) koine.Conclusion {
	if l.g.fault != "" {
		return koine.Conclusion{Outcome: koine.Failure, Err: &Stopped{Why: l.g.fault}}
	}
	if p == 0 {
		// Either the pass was suppressed — awaiting a withheld pass is
		// asking what a parent concluded about something it never
		// received — or the host would not open one.
		if c, ok := l.g.passing.AwaitedConclusion(); ok {
			return c // the one answer, single-sourced beside the gate itself
		}
		return koine.Conclusion{
			Outcome: koine.Failure,
			Err:     &Stopped{Why: "this station awaited a passage the host never opened"},
		}
	}
	raw := l.g.host.ValuePoll(uint64(p))
	if len(raw) == 0 {
		l.g.faultBelowTheLine("the host answered a pass-up with nothing")
		return koine.Conclusion{Outcome: koine.Failure, Err: &Stopped{Why: l.g.fault}}
	}
	answer, err := DecodeAnswer(raw)
	if err != nil {
		l.g.faultBelowTheLine("the host answered a pass-up with a frame this build cannot read: " + err.Error())
		return koine.Conclusion{Outcome: koine.Failure, Err: &Stopped{Why: l.g.fault}}
	}

	switch {
	case answer.Breach():
		// A breach the parent reported is a VALUE here, always. Asking
		// is declaring: a station that awaited said it would handle
		// what came back.
		return koine.Conclusion{
			Outcome: koine.Failure,
			Err:     &Variant{Name: answer.Error, Status: answer.Status},
		}
	case answer.Stoppage() && isUnfilled(answer.Error):
		// A seat nobody filled. Determinate, and not a malfunction —
		// so it does NOT fault the run.
		return koine.Conclusion{Outcome: koine.Failure, Err: &Unfilled{Why: answer.Error}}
	case answer.Stoppage():
		l.g.faultBelowTheLine("the parent chain answered status " +
			strconv.Itoa(answer.Status) + ": " + answer.Error)
		return koine.Conclusion{Outcome: koine.Failure, Err: &Stopped{Why: l.g.fault}}
	}

	// A parent that concluded plainly answers with no value, and for a
	// pass-up that is the ORDINARY case — a controller that enriched and
	// stored has nothing further to say. The exchange path treats an empty
	// filled answer as a stoppage because a fulfiller with nothing to say
	// breaches; a parent step is not a fulfiller, and reading it the same
	// way would turn every quiet success into a fault.
	return koine.Conclusion{Outcome: koine.Success}
}

// isUnfilled recognises the unfilled-seat answer. See UnfilledPrefix: this is
// isUnfilled recognises the engine's own ErrNotImplemented text (see
// UnfilledPrefix — the settled sentinel, pinned with #210's 501 shape),
// matched as a prefix because the host's sentence names the namespace after
// it.
func isUnfilled(msg string) bool {
	return len(msg) >= len(UnfilledPrefix) && msg[:len(UnfilledPrefix)] == UnfilledPrefix
}

// offer renders one pass-up or withhold. The object rides Intent as the
// domain object itself with its type key spliced flat — the YieldFrame
// convention, because the host already reads a yield's type that way.
func (l lineage) offer(kind string, u koine.Utterance) []byte {
	frame := ExchangeFrame{Type: kind}
	if u != nil {
		speakable, ok := u.(Speakable)
		if !ok {
			panic("koine/wire: a station passed up something no stratum generated — the parent chain carries named domain objects, and there is no second channel to carry anything else")
		}
		var w codec.Writer
		w.BeginObject()
		w.Key(YieldTypeKey)
		w.String(speakable.EventType())
		speakable.EncodeFields(&w)
		w.EndObject()
		frame.Intent = append([]byte(nil), w.Bytes()...)
	}
	data, err := frame.MarshalJSON()
	if err != nil {
		panic("koine/wire: a pass-up frame could not render itself: " + err.Error())
	}
	return data
}
