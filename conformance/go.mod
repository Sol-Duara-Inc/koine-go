// This is a SEPARATE module on purpose.
//
// koine-go's own module has no dependencies and no go.sum, and an
// architecture test holds it there (A6). The crossing test needs the
// engine — the real koinehost, and wazero underneath it — so it lives in a
// module of its own. Go excludes a subdirectory that has its own go.mod from
// the parent's package patterns, so `go build ./...`, `go vet ./...` and
// `go test ./...` at the repository root do not see this directory at all,
// and the zero-dependency law is untouched.
//
// The replace paths assume the two repositories are checked out side by
// side, which is what CI does:
//
//	<workspace>/koine-go
//	<workspace>/conduit-go
//
// A replace is required rather than a version: conduit-go's module path is
// github.com/solduara/conduit-go while its repository is
// github.com/Sol-Duara-Inc/conduit-go, so `go get` cannot resolve it, and it
// is private besides.
module github.com/sol-duara-inc/koine-go/conformance

// The go directive tracks the ENGINE's, not the SDK's: this module compiles
// the engine's source, so it cannot ask for less than the engine asks for.
// The SDK itself stays at its own floor, and nothing here reaches back into
// it.
go 1.26

require (
	github.com/sol-duara-inc/koine-go v0.0.0
	github.com/solduara/conduit-go v0.0.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1 // indirect
	github.com/sol-duara-inc/conduit-layers-go v0.3.0 // indirect
	github.com/tetratelabs/wazero v1.12.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/sol-duara-inc/koine-go => ..

replace github.com/solduara/conduit-go => ../../conduit-go
