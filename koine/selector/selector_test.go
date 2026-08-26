package selector_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/koine/selector"
)

func TestSelector_ListPreservesOrder(t *testing.T) {
	src := []selector.Selector{
		selector.Event("dev.cdevents.build.finished"),
		selector.Resolved("dev.cdevents.deployment"),
		selector.Absent(selector.Event("dev.cdevents.testcaserun.started")),
	}
	out := selector.List(src...)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
	for i := range src {
		if out[i] != src[i] {
			t.Fatalf("order broken at %d: %+v != %+v", i, out[i], src[i])
		}
	}
	// A declaration cannot be mutated through a retained argument slice.
	src[0].Type = "mutated"
	if out[0].Type != "dev.cdevents.build.finished" {
		t.Fatalf("List shares backing storage with its arguments: %+v", out[0])
	}
}

func TestSelector_ResolvedIsOutcomeBlind(t *testing.T) {
	b, err := json.Marshal(selector.Resolved("dev.cdevents.deployment"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(strings.ToLower(s), "outcome") {
		t.Fatalf("a resolved selector must carry no outcome filter, got %s", s)
	}
	if !strings.Contains(s, `"mode":"resolved"`) {
		t.Fatalf("serialized form lost the mode: %s", s)
	}
}

func TestSelector_AbsentIsDistinct(t *testing.T) {
	e := selector.Event("dev.cdevents.build.finished")
	a := selector.Absent(e)
	if a.Mode != selector.ModeAbsent {
		t.Fatalf("Absent mode = %q", a.Mode)
	}
	if e.Mode != selector.ModeEvent {
		t.Fatalf("Absent mutated its argument: %+v", e)
	}
	if a == e {
		t.Fatal("absence must be distinct from arrival")
	}
}

func TestSelector_AnchorRoundTrips(t *testing.T) {
	base := selector.Event("dev.cdevents.pipelinerun.started")
	at := base.At("kickoff")
	if at.Anchor != "kickoff" {
		t.Fatalf("Anchor = %q", at.Anchor)
	}
	if base.Anchor != "" {
		t.Fatalf("At mutated its receiver: %+v", base)
	}
}

func TestSelector_JSONRoundTrips(t *testing.T) {
	cases := []selector.Selector{
		selector.Event("dev.cdevents.build.finished"),
		selector.Event("dev.cdevents.pipelinerun.started").At("kickoff"),
		selector.Resolved("dev.cdevents.deployment"),
		selector.Absent(selector.Event("dev.cdevents.testcaserun.started")),
	}
	for _, c := range cases {
		b, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		if c.Anchor == "" && strings.Contains(string(b), `"anchor"`) {
			t.Fatalf("empty anchor must be omitted: %s", b)
		}
		var back selector.Selector
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(c, back) {
			t.Fatalf("round trip lost data: %+v -> %s -> %+v", c, b, back)
		}
	}
}
