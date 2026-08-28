// This test lives INSIDE the package, because what it checks has no exported
// surface and should not grow one: the host reaches guest memory through
// alloc and through nothing else, and a knob a guest could turn is a knob a
// guest could turn.
package wire

import "testing"

// TestWire_TheArenaIsReclaimedAtEveryDelivery pins the discipline behind the
// alloc export. The host asks for room before every frame it pushes — the
// delivery, and every exchange answer during the run — and a bump allocator
// that never gave anything back would fill up.
//
// The host's own Run instantiates a fresh module per delivery, so today
// nothing would notice. That is not a reason to leak: a guest that only
// works because its caller throws it away after one use is a guest with a
// fuse in it, and reclaiming costs one assignment.
func TestWire_TheArenaIsReclaimedAtEveryDelivery(t *testing.T) {
	var a arena

	if a.used() != 0 {
		t.Fatal("a fresh arena is already holding something")
	}
	first := a.alloc(1024)
	if len(first) != 1024 || a.used() != 1024 {
		t.Fatalf("alloc(1024) gave %d bytes, arena holds %d", len(first), a.used())
	}
	second := a.alloc(16)
	if a.used() != 1040 {
		t.Fatalf("two allocations hold %d", a.used())
	}
	// The two do not overlap: a bump allocator that handed out the same
	// bytes twice would have the host writing a delivery over an answer.
	for i := range first {
		first[i] = 'a'
	}
	for i := range second {
		second[i] = 'b'
	}
	for i, c := range first {
		if c != 'a' {
			t.Fatalf("the first allocation was overwritten at byte %d", i)
		}
	}

	a.reset()
	if a.used() != 0 {
		t.Fatalf("reset left %d bytes held", a.used())
	}

	// And it can be filled and reclaimed forever, which is the whole
	// point: the number of deliveries an instance survives is not bounded
	// by its memory.
	for i := 0; i < 1000; i++ {
		if a.alloc(ArenaCapacity/2) == nil {
			t.Fatalf("delivery %d could not be served", i)
		}
		a.reset()
	}
}

// TestWire_AnOversizeAllocationIsRefusedNotWrappedAround pins what happens
// at the edge. Answering an address the arena does not own would have the
// host writing into whatever is there; answering nil is how the guest says
// no, and the host reads a zero address as a refusal.
func TestWire_AnOversizeAllocationIsRefusedNotWrappedAround(t *testing.T) {
	var a arena
	if a.alloc(ArenaCapacity+1) != nil {
		t.Fatal("the arena served more than it has")
	}
	if a.used() != 0 {
		t.Fatalf("a refused allocation still moved the pointer to %d", a.used())
	}
	if a.alloc(-1) != nil {
		t.Fatal("the arena served a negative length")
	}

	if a.alloc(ArenaCapacity) == nil {
		t.Fatal("the arena would not serve its whole capacity")
	}
	if a.alloc(1) != nil {
		t.Fatal("the arena served one byte past full")
	}

	// A zero-length request is not a refusal: the host writes nothing at
	// the address, and reading the answer as a refusal would fail a
	// delivery that simply had nothing in it.
	a.reset()
	if a.alloc(0) == nil {
		t.Fatal("a zero-length allocation was refused")
	}
	if a.used() != 0 {
		t.Fatalf("a zero-length allocation consumed %d bytes", a.used())
	}
}
