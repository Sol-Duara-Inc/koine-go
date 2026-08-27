// Package fixtures holds the committed output of koinegen: the generated
// strata under strata/ and the stations under station/ that the extractor
// reads and the harness drives.
//
// The goldens are committed, and a full regeneration must produce zero diff.
// That is not tidiness — it is the proof that generation is a function of
// the registry alone. A floor package's bytes do not move when a vendor or a
// customer is added, because a floor namespace's file never mentions what
// extends it.
//
// Regenerate with `go generate ./cmd/koinegen/fixtures/`.
package fixtures

//go:generate go run github.com/sol-duara-inc/koine-go/cmd/koinegen generate -registry ../testdata/registry -out ./strata -pkgbase github.com/sol-duara-inc/koine-go/cmd/koinegen/fixtures/strata
//go:generate go run github.com/sol-duara-inc/koine-go/cmd/koinegen manifest -registry ../testdata/registry -station ./station -koine DeploymentSteward -o ./manifests/deployment-steward.json
//go:generate go run github.com/sol-duara-inc/koine-go/cmd/koinegen manifest -registry ../testdata/registry -station ./station -koine DeploymentAuditor -o ./manifests/deployment-auditor.json
//go:generate go run github.com/sol-duara-inc/koine-go/cmd/koinegen manifest -registry ../testdata/registry -station ./station -koine DeploymentRehearsal -o ./manifests/deployment-rehearsal.json
