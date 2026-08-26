// Package koine is the author's side of the Koine paradigm: stations declare
// what they await, arrivals store until the shape of complete is satisfied,
// and resolving emits — speech, not side effect. Everything below the line
// (envelopes, chains, actors, credentials, storage-by-emission) belongs to
// the host and is structurally unreachable from here.
//
// The design law of this surface is the domain of control: the perspective
// being served reaches everything they can see, as if they were there making
// the decisions — absolute ruler of their kingdom. Inside your projection,
// everything is completely yours: every field of your Delivery, every verb on
// your base, every exchange your stratum can speak. The border of the kingdom
// is the type system itself — what your stratum does not contain does not
// exist for you — and no verb you can reach ever asks permission or fails
// silently. There are no runtime permission checks inside a station body: if
// you can write the call, the call speaks.
//
// Stations are authored by humans and by AIs alike. The surface is therefore
// deliberately small, declarative, and derivable: an agent holding only these
// contracts can emit a correct station, and the manifest that describes a
// station is derived from its code — a declaration can never lie about the
// body it describes.
//
// A Resolve is a pure function from delivery to utterances. Inside it there
// are only two verbs: calculating or emitting — the data is already there.
package koine
