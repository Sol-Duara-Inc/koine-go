//go:build !tinygo.wasm

package main

import "os"

// Off the wasm target there are no exports to declare and no host to
// declare them to. The fixture still compiles — `go build ./...` and
// `go vet ./...` cover it like everything else — and running it says so
// rather than pretending to be a guest.
func main() {
	os.Stderr.WriteString("steward: this is a wasm guest fixture, not a program — " +
		"build it with `tinygo build -target wasm-unknown ./fixtures/guest/steward` " +
		"and load it with the engine's koinehost\n")
	os.Exit(1)
}
