package koine_test

import (
	"strings"
	"testing"

	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/wire"
)

func TestPassUp_NilLineageAnswersUnfilled(t *testing.T) {
	station := &steward{}
	if !koine.Construct(station, koine.Standing{Lineage: nil}) {
		t.Fatal("Construct refused a station with an empty parent")
	}
	offered := deployRecorded{Artifact: "payments-api"}

	passage := station.PassUp(offered)
	if passage != 0 {
		t.Fatalf("PassUp returned passage %d, want the empty passage 0", passage)
	}
	station.Withhold(offered)
	got := station.Await(passage)

	if got.Outcome != koine.Failure {
		t.Fatalf("Await returned outcome %v, want Failure", got.Outcome)
	}
	if got.Err == nil || !strings.HasPrefix(got.Err.Error(), wire.UnfilledPrefix) {
		t.Fatalf("Await returned error %v, want prefix %q", got.Err, wire.UnfilledPrefix)
	}
}

type recordingLineage struct {
	offered    koine.Utterance
	awaited    koine.Passage
	withheld   koine.Utterance
	conclusion koine.Conclusion
}

func (l *recordingLineage) PassUp(u koine.Utterance) koine.Passage {
	l.offered = u
	return 42
}

func (l *recordingLineage) AwaitPass(p koine.Passage) koine.Conclusion {
	l.awaited = p
	return l.conclusion
}

func (l *recordingLineage) Withhold(u koine.Utterance) {
	l.withheld = u
}

func TestPassUp_RealLineageIsUsed(t *testing.T) {
	lineage := &recordingLineage{conclusion: koine.Conclusion{Outcome: koine.Success}}
	station := &steward{}
	if !koine.Construct(station, koine.Standing{Lineage: lineage}) {
		t.Fatal("Construct refused a station with a real parent")
	}
	offered := deployRecorded{Artifact: "payments-api"}

	passage := station.PassUp(offered)
	got := station.Await(passage)
	station.Withhold(offered)

	if passage != 42 || lineage.offered != offered {
		t.Fatalf("PassUp returned %d and offered %#v", passage, lineage.offered)
	}
	if got != lineage.conclusion || lineage.awaited != passage {
		t.Fatalf("Await returned %#v after passage %d", got, lineage.awaited)
	}
	if lineage.withheld != offered {
		t.Fatalf("Withhold received %#v", lineage.withheld)
	}
}
