// Command koinegen turns the schema registry into data strata and extracts a
// station's registration manifest from its code.
//
// Two subcommands, one law between them: everything koinegen writes is
// DERIVED. There is no path here that reads a hand-written manifest and no
// flag that would let one in — a declaration that a human typed could lie
// about the body it describes, so the tool refuses the whole idea by not
// implementing it (A3).
//
//	koinegen generate -registry DIR -out DIR -pkgbase IMPORTPATH
//	koinegen manifest -registry DIR -station DIR [-koine Name] [-o FILE]
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

const usage = `koinegen — data strata and registration manifests, derived from the registry

  koinegen generate -registry DIR -out DIR -pkgbase IMPORTPATH
  koinegen manifest -registry DIR -station DIR [-koine NAME] [-o FILE]
`

func run(args []string, stdout *os.File) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "generate":
		return runGenerate(args[1:])
	case "manifest":
		return runManifest(args[1:], stdout)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return nil
	}
	return fmt.Errorf("koinegen: unknown subcommand %q\n\n%s", args[0], usage)
}

func runGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	registry := fs.String("registry", "", "directory holding the schema registry")
	out := fs.String("out", "", "directory the generated packages are written to")
	pkgBase := fs.String("pkgbase", "", "import path the generated packages live under")
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch {
	case *registry == "":
		return errors.New("koinegen generate: -registry is required")
	case *out == "":
		return errors.New("koinegen generate: -out is required")
	case *pkgBase == "":
		return errors.New("koinegen generate: -pkgbase is required")
	}
	reg, err := LoadRegistry(*registry)
	if err != nil {
		return err
	}
	written, err := Generate(reg, *pkgBase, *out)
	if err != nil {
		return err
	}
	for _, path := range written {
		fmt.Fprintln(os.Stderr, "koinegen: wrote", filepath.Join(*out, filepath.FromSlash(path)))
	}
	return nil
}

func runManifest(args []string, stdout *os.File) error {
	fs := flag.NewFlagSet("manifest", flag.ContinueOnError)
	registry := fs.String("registry", "", "directory holding the schema registry")
	station := fs.String("station", "", "directory holding the station's Go source")
	name := fs.String("koine", "", "the station type to extract, when the package holds more than one")
	outFile := fs.String("o", "", "write the manifest here instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch {
	case *registry == "":
		return errors.New("koinegen manifest: -registry is required")
	case *station == "":
		return errors.New("koinegen manifest: -station is required")
	}
	reg, err := LoadRegistry(*registry)
	if err != nil {
		return err
	}
	found, err := Extract(reg, *station)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(found))
	for n := range found {
		names = append(names, n)
	}
	sort.Strings(names)
	switch {
	case len(names) == 0:
		return fmt.Errorf("koinegen manifest: %s declares no station — a manifest is derived from a body, and there is no body here", *station)
	case *name == "" && len(names) > 1:
		return fmt.Errorf("koinegen manifest: %s declares %d stations (%s) — name one with -koine", *station, len(names), strings.Join(names, ", "))
	case *name == "":
		*name = names[0]
	}
	m, ok := found[*name]
	if !ok {
		return fmt.Errorf("koinegen manifest: %s declares no station named %q; it declares %s", *station, *name, strings.Join(names, ", "))
	}
	data, err := m.JSON()
	if err != nil {
		return err
	}
	if *outFile == "" {
		_, err := stdout.Write(data)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*outFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(*outFile, data, 0o644)
}
