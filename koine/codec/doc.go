// Package codec is the reflection-free JSON surface generated stratum code
// is written against. It exists so codegen can emit marshal/unmarshal that
// names every field in source — no reflection, no struct tags read at
// runtime — and the guest target stays open (A6: the SDK embeds anywhere Go
// compiles, TinyGo included).
//
// It is deliberately small and deliberately not general: it reads and writes
// the shapes generated code produces, and refuses everything else by name.
// Nothing here is an author surface — a station body never sees a Reader.
package codec
