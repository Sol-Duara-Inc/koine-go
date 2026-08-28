// This one test lives INSIDE the package, because what it checks is a guard
// that has no exported surface and should not grow one: a knob a guest could
// reach is a knob a guest could turn.
package wire

import (
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/koine"
)

// TestWire_AHostThatNeverAnswersIsLoud pins the guard behind the wait. It is
// not a timeout and it is not a budget — budgets are the engine's, minted
// from the manifest's topology — it is the difference between a host that is
// slow and a host that is not answering. A guest that has asked a million
// times and been told "pending" a million times is talking to something
// broken, and it says so rather than spinning in the sandbox until someone
// notices.
//
// A host that suspends and resumes the guest across the boundary never
// reaches one iteration.
func TestWire_AHostThatNeverAnswersIsLoud(t *testing.T) {
	saved := pollLimit
	pollLimit = 8
	defer func() { pollLimit = saved }()

	h := &alwaysPending{}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("the guest waited forever without saying so")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "not answering") {
			t.Fatalf("the trap said %v", r)
		}
		if h.polls != pollLimit {
			t.Errorf("the guest asked %d times, want %d", h.polls, pollLimit)
		}
	}()
	broker{host: h}.Await(koine.Token(1))
}

type alwaysPending struct{ polls int }

func (h *alwaysPending) Yield([]byte) bool      { return true }
func (h *alwaysPending) Exchange([]byte) []byte { return nil }
func (h *alwaysPending) AckPoll(uint64) []byte  { return nil }

func (h *alwaysPending) ValuePoll(uint64) []byte {
	h.polls++
	data, err := ValueFrame{Wire: Version, State: StatePending}.MarshalJSON()
	if err != nil {
		panic(err)
	}
	return data
}

// TestWire_AnUnknownPollStateIsRefused pins that the state vocabulary is
// closed. A host that invents a fourth word is not speaking this contract,
// and the guest says which word rather than guessing at what was meant.
func TestWire_AnUnknownPollStateIsRefused(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("the guest accepted a state this contract does not admit")
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "maybe") {
			t.Fatalf("the trap said %v", r)
		}
	}()
	broker{host: statePolls("maybe")}.Await(koine.Token(1))
}

type statePolls string

func (s statePolls) Yield([]byte) bool      { return true }
func (s statePolls) Exchange([]byte) []byte { return nil }
func (s statePolls) AckPoll(uint64) []byte  { return nil }

func (s statePolls) ValuePoll(uint64) []byte {
	data, err := ValueFrame{Wire: Version, State: string(s)}.MarshalJSON()
	if err != nil {
		panic(err)
	}
	return data
}
