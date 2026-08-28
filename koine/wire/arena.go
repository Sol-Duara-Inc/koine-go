package wire

// Guest linear memory belongs to the guest, and the alloc export is the only
// door into it. The host calls alloc before every frame it pushes — the
// delivery at the start of a run, and every exchange answer during one — and
// writes at the address alloc answers with.
//
// The allocator is a bump pointer over one fixed region, and it is RECLAIMED
// at the boundary of every delivery. That matters even though the host's own
// Run instantiates a fresh module each time: a guest that only works because
// its caller throws it away after one use is a guest with a fuse in it, and
// the next caller to reuse an instance would find it. Reclaiming costs one
// assignment.
//
// Nothing here frees individually. Everything the host writes is read and
// copied out inside the same resolve, so a mark at the start and a reset at
// the end is the whole discipline.

// ArenaCapacity is the size of the region the host writes frames into. It
// bounds one delivery plus the exchange answers spoken during it. The host
// runs guests under a memory limit of its own (256 pages by default), and
// this sits well inside it.
const ArenaCapacity = 1 << 20

// arena is a bump allocator over a fixed region. The region is allocated
// once, on the first alloc, so a guest that is only ever asked for its
// manifest never pays for it.
type arena struct {
	buf []byte
	off int
}

// alloc answers a slice of n bytes, or nil when the region cannot hold it.
// The caller turns nil into a zero address, which the host reads as a
// refusal.
func (a *arena) alloc(n int) []byte {
	if n < 0 {
		return nil
	}
	if a.buf == nil {
		a.buf = make([]byte, ArenaCapacity)
	}
	if n == 0 {
		// A zero-length allocation still needs a real address: the host
		// writes nothing at it, but it must not read the answer as a
		// refusal.
		if a.off >= len(a.buf) {
			return nil
		}
		return a.buf[a.off:a.off:a.off]
	}
	if a.off+n > len(a.buf) {
		return nil
	}
	out := a.buf[a.off : a.off+n : a.off+n]
	a.off += n
	return out
}

// reset reclaims the whole region. Every caller of alloc has copied what it
// needed out by the time this runs.
func (a *arena) reset() { a.off = 0 }

// used is how much of the region is currently handed out. It exists for the
// test that proves reclamation happens, and for a diagnostic line.
func (a *arena) used() int { return a.off }
