// Command tinygocheck exists to be compiled, not run: it references the SDK
// surface from a main package so both supported toolchains (standard Go and
// TinyGo — E-A as amended) prove they can build author code against it.
//
// K1 widened what it references. The generated strata and their hand-rolled
// codec are here on purpose: "reflection-free marshal/unmarshal so the guest
// target stays open" is a claim, and the only way to hold it is to compile
// generated code with the toolchain that would refuse reflection.
package main

import (
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/deployment"
	"github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata/payments"
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/codec"
	"github.com/sol-duara-inc/koine-go/koine/selector"
)

func main() {
	awaits := selector.List(
		selector.Event("dev.cdevents.build.finished"),
		selector.Resolved("dev.cdevents.deployment").At("deploy"),
		selector.Absent(selector.Event("dev.cdevents.testcaserun.started")),
		deployment.Resolved(),
	)
	id := koine.Identity{Group: "sol-duara", Author: "solo-duara-agent", Name: "tinygocheck"}
	if len(awaits) != 4 || id.String() == "" || koine.DefaultAllAwaited == nil {
		panic("koine: the surface did not survive compilation")
	}

	// A whole customer-stratum object, seeded at the deepest extension and
	// round-tripped through the generated codec.
	built := payments.SeedPaymentsBuildFinished("payments-api", koine.Success, "sha256:abc", "agent-7", 42, "cc-1180", true)
	data, err := built.MarshalJSON()
	if err != nil || len(data) == 0 {
		panic("koine: generated marshal did not survive compilation")
	}
	var back payments.PaymentsBuildFinished
	if err := back.UnmarshalJSON(data); err != nil || back.CostCenter != "cc-1180" {
		panic("koine: generated unmarshal did not survive compilation")
	}

	// And the codec on its own, since generated code is written against it.
	var w codec.Writer
	w.BeginObject()
	w.Key("outcome")
	w.String(string(koine.Failure))
	w.EndObject()
	if len(w.Bytes()) == 0 {
		panic("koine: the codec did not survive compilation")
	}

	var d koine.Delivery = deployment.ResolvedDelivery{Outcome: koine.Failure}
	if d == nil {
		panic("koine: a generated delivery did not survive compilation")
	}
}
