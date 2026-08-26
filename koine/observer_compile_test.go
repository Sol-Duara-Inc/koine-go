package koine_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestObserver_CannotResolveChain pins plane discipline as reachability: an
// observer station speaking a chain verb DOES NOT COMPILE. The verb does not
// exist from that position — removed affordance, not guarded path. The test
// builds a real foreign module against this repo twice: the violation must
// fail naming ResolveChain, and the execution-stratum control must succeed —
// without the passing control, the failure would prove nothing.
func TestObserver_CannotResolveChain(t *testing.T) {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller info")
	}
	repoRoot := filepath.Dir(filepath.Dir(self)) // koine/ -> repo root

	build := func(t *testing.T, body string) (string, error) {
		t.Helper()
		dir := t.TempDir()
		gomod := "module observercheck\n\ngo 1.23\n\nrequire github.com/sol-duara-inc/koine-go v0.0.0\n\nreplace github.com/sol-duara-inc/koine-go => " + repoRoot + "\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	const violation = `package main

import "github.com/sol-duara-inc/koine-go/koine"

type spy struct{ koine.ObserverBase }

func main() {
	var s spy
	s.ResolveChain(koine.Success)
}
`
	out, err := build(t, violation)
	if err == nil {
		t.Fatal("an observer station calling ResolveChain compiled — the plane wall is down")
	}
	if !strings.Contains(out, "ResolveChain") {
		t.Fatalf("compile failed for the wrong reason:\n%s", out)
	}

	const control = `package main

import "github.com/sol-duara-inc/koine-go/koine"

type steward struct{ koine.ExecutionBase }

func main() {
	var s steward
	if false {
		s.ResolveChain(koine.Success, koine.Evidence{Ref: "run/x"})
		s.ExtendChain(0)
	}
	_ = s
}
`
	if out, err := build(t, control); err != nil {
		t.Fatalf("the execution-stratum control failed to compile — the violation result proves nothing:\n%s", out)
	}
}
