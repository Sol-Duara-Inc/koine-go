// Command tinygocheck exists to be compiled, not run: it references the SDK
// surface from a main package so both supported toolchains (standard Go and
// TinyGo — E-A as amended) prove they can build author code against it.
package main

import (
	"github.com/sol-duara-inc/koine-go/koine"
	"github.com/sol-duara-inc/koine-go/koine/selector"
)

func main() {
	awaits := selector.List(
		selector.Event("dev.cdevents.build.finished"),
		selector.Resolved("dev.cdevents.deployment").At("deploy"),
		selector.Absent(selector.Event("dev.cdevents.testcaserun.started")),
	)
	id := koine.Identity{Group: "sol-duara", Author: "solo-duara-agent", Name: "tinygocheck"}
	if len(awaits) != 3 || id.String() == "" || koine.DefaultAllAwaited == nil {
		panic("koine: the surface did not survive compilation")
	}
}
