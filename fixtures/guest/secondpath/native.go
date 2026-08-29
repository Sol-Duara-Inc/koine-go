//go:build !tinygo.wasm

package main

import "os"

// Off the wasm target there are no exports to declare, and the offence this
// fixture exists to commit cannot be committed.
func main() {
	os.Stderr.WriteString("secondpath: this is a NEGATIVE wasm guest fixture, not a program — " +
		"build it with `tinygo build -target wasm-unknown ./fixtures/guest/secondpath` " +
		"and watch the engine's koinehost refuse it by name\n")
	os.Exit(1)
}
