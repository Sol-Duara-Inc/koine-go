package main

import (
	"errors"
	"testing"

	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/station"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/manifest"
	koinetest "github.com/sol-duara-inc/koine-go/koine/testing"
)

// This is A3's gate: the extractor's output is pinned to what the station's
// body ACTUALLY does. The manifest is the static truth — everything the body
// can say on any path — and a run is one path through it, so the relation
// asserted here is containment with exact agreement on whatever the run
// touched. Every path of every fixture station is driven below, so the two
// readings cover each other completely.
//
// All three chain roles are compared word for word. Until 2026-08-27 the
// third was invisible to a run — koine.Detach was read from source and
// nowhere else, so a wrong "detached" could pass this gate. Detach now tells
// the host below the handle as well, and the comparison below is exact.

// behaviour is one driven path and what it turned out to say.
type behaviour struct {
	name    string
	station koine.Koine
	out     koinetest.Out
}

func drivenPaths(t *testing.T) map[string][]behaviour {
	t.Helper()
	lastGood := deployment.SeedDeploymentFinished("payments-api", koine.Success, "sha256:good", "prod")
	noted := deployment.SeedDeploymentRecorded("sha256:bad")

	failed := deployment.ResolvedDelivery{
		Subject: "payments-api", Outcome: koine.Failure,
		ArtifactID: "sha256:bad", Environment: "prod",
	}
	succeeded := deployment.ResolvedDelivery{
		Subject: "payments-api", Outcome: koine.Success,
		ArtifactID: "sha256:fine", Environment: "prod",
	}

	return map[string][]behaviour{
		"DeploymentSteward": {
			{
				name:    "the success branch",
				station: station.DeploymentSteward{},
				out: koinetest.Run(station.DeploymentSteward{},
					koinetest.Deliver(succeeded),
					koinetest.Exchange("history.last", lastGood)),
			},
			{
				name:    "the failure branch, answered",
				station: station.DeploymentSteward{},
				out: koinetest.Run(station.DeploymentSteward{},
					koinetest.Deliver(failed),
					koinetest.Exchange("history.last", lastGood)),
			},
			{
				name:    "the failure branch, variant",
				station: station.DeploymentSteward{},
				out: koinetest.Run(station.DeploymentSteward{},
					koinetest.Deliver(failed),
					koinetest.ExchangeFails("history.last", errors.New("nothing good to fall back to"))),
			},
		},
		"DeploymentAuditor": {
			{
				name:    "the only branch",
				station: station.DeploymentAuditor{},
				out: koinetest.Run(station.DeploymentAuditor{},
					koinetest.Deliver(failed),
					koinetest.Exchange("history.last", lastGood),
					koinetest.Exchange("ledger.note", noted)),
			},
		},
		// The rehearsal binds its handle and its seat before speaking
		// through them. Driving it here is what keeps the gate honest
		// about the spelling: an analyzer that only read chained calls
		// would declare its inline exchange blocking, and this comparison
		// would catch it.
		"DeploymentRehearsal": {
			{
				name:    "the variant branch",
				station: station.DeploymentRehearsal{},
				out: koinetest.Run(station.DeploymentRehearsal{},
					koinetest.Deliver(failed),
					koinetest.ExchangeFails("history.last", errors.New("nothing good to fall back to"))),
			},
			{
				name:    "the answered branch",
				station: station.DeploymentRehearsal{},
				out: koinetest.Run(station.DeploymentRehearsal{},
					koinetest.Deliver(failed),
					koinetest.Exchange("history.last", lastGood),
					koinetest.Exchange("ledger.note", noted)),
			},
		},
	}
}

// TestManifest_ExtractorMatchesBehaviour is the conformance test A3 names:
// the declaration can never lie about the code, and this is what proves it.
func TestManifest_ExtractorMatchesBehaviour(t *testing.T) {
	declared := extractFixtures(t)
	driven := drivenPaths(t)
	witnessed := map[manifest.Consumption]bool{}

	for name, paths := range driven {
		m, ok := declared[name]
		if !ok {
			t.Fatalf("the extractor derived no manifest for %s", name)
		}

		emits := map[string]bool{}
		for _, e := range m.Koine.Emits {
			emits[e.Type] = true
		}
		exchanges := map[string]manifest.Consumption{}
		for _, ex := range m.Koine.Exchanges {
			exchanges[ex.Name] = ex.Consumption
		}
		seats := map[string]bool{}
		for _, s := range m.Koine.Seats {
			seats[s.Seat] = true
		}

		spokenTypes := map[string]bool{}
		touched := map[string]bool{}

		for _, b := range paths {
			t.Run(name+": "+b.name, func(t *testing.T) {
				// The claim the manifest carries is the claim the station
				// answers with — same source, checked from both ends.
				if m.Claim.Group != b.out.Identity.Group || m.Claim.Author != b.out.Identity.Author || m.Claim.Name != b.out.Identity.Name {
					t.Errorf("the manifest claims %#v; the station answers %s", m.Claim, b.out.Identity)
				}
				if len(m.Koine.Awaits) != len(b.out.Awaits) {
					t.Fatalf("the manifest declares %d awaits; the station lists %d", len(m.Koine.Awaits), len(b.out.Awaits))
				}
				for i, a := range m.Koine.Awaits {
					got := b.out.Awaits[i]
					if a.Type != got.Type || a.Mode != string(got.Mode) || a.Anchor != got.Anchor {
						t.Errorf("await %d: manifest %#v, station %#v", i, a, got)
					}
				}

				for _, u := range b.out.Utterances {
					typed, ok := u.(interface{ EventType() string })
					if !ok {
						t.Fatalf("a generated utterance does not name its event type: %#v", u)
					}
					spokenTypes[typed.EventType()] = true
					if !emits[typed.EventType()] {
						t.Errorf("the body spoke %q, which the manifest does not declare", typed.EventType())
					}
				}

				for _, ex := range b.out.Exchanges {
					touched[ex.Name] = true
					witnessed[ex.Consumption] = true
					how, ok := exchanges[ex.Name]
					if !ok {
						t.Errorf("the body spoke exchange %q, which the manifest does not declare", ex.Name)
						continue
					}
					if how != ex.Consumption {
						t.Errorf("%s: the manifest reads %q; the run observed %q",
							ex.Name, how, ex.Consumption)
					}
					if !seats[ex.Seat] {
						t.Errorf("the body spoke at seat %q, which the manifest does not require", ex.Seat)
					}
				}
			})
		}

		// The other direction: nothing declared is a fiction. Driving every
		// branch of these fixtures reaches every emit and every exchange the
		// extractor claims, so an over-declaration is caught too.
		for _, e := range m.Koine.Emits {
			if !spokenTypes[e.Type] {
				t.Errorf("%s declares emit %q that no driven path ever speaks", name, e.Type)
			}
		}
		for _, ex := range m.Koine.Exchanges {
			if !touched[ex.Name] {
				t.Errorf("%s declares exchange %q that no driven path ever speaks", name, ex.Name)
			}
		}
	}

	// And the gate can tell all three roles apart. A comparison that
	// collapsed two of them would pass every assertion above while letting
	// a wrong word through — which is exactly what it did until Detach
	// began telling the host as well as the analyzer.
	for _, how := range []manifest.Consumption{manifest.Inline, manifest.Concurrent, manifest.Detached} {
		if !witnessed[how] {
			t.Errorf("no driven path ever WITNESSED %q — the gate cannot catch a manifest that claims it", how)
		}
	}
}
