# Contributing

## Setup

`nix develop` gives you Go, golangci-lint, goreleaser, just, uv, Python 3.13,
Node and TypeScript, with the library path set for the Python wheels. Without
Nix you need Go 1.26 or newer; the Python and Node checks are optional.

```sh
just check          # vet, lint, Go tests, Python tests, web tests
just build          # dist/mufradat
```

or, by hand:

```sh
go vet ./...
go run ./cmd/nocomments .
go test -race ./...
(cd python && python -m pytest -q)
(cd internal/review && node --test jstest/*.test.js && tsc -p .)
```

## Rules

- **No comments in the code.** Names, small functions and tests carry the
  explanation. `go run ./cmd/nocomments .` fails on any comment except
  `//go:` directives and JSDoc type annotations; CI runs it. Explanations that
  matter belong in `docs/` or the commit message.
- Keep the layering in [`docs/architecture.md`](docs/architecture.md): small
  interfaces defined by the consumer, no `init()`, no package-level mutable
  state, the environment only through `env.Env`, subprocesses only through
  `proc.Runner`, `os.Exit` only in `main`.
- The review page is plain ES modules with JSDoc types and no build step. It is
  type-checked with `tsc --checkJs` in strict mode and the pure modules are
  tested with `node --test`.
- Tests that need the outside world use fakes: `internal/cli` registers fake
  `uv`, `python`, `camel_data` and `xdg-open` programs and strict fake Mistral
  and Google servers for its `testscript` files (`internal/cli/testdata/script`).
  The fakes check the method, path, query, headers and exact body, like the
  real APIs do, and a script can assert exit codes with `exits CODE command`.
- Real network runs are opt-in and never part of `go test`: the tests behind the
  `integration` build tag call the real APIs (see
  [`INTEGRATION_TESTS.md`](INTEGRATION_TESTS.md)).

## Changing the word list

`data/` is generated. Edit `data/curation/essentials.tsv` or `overrides.tsv`, or
the code in `internal/rankgen`, and run `go run ./cmd/mkranking` (after
`mufradat setup`). Commit the regenerated files with the change. See
[`data/SOURCES.md`](data/SOURCES.md) for the sources and their licences; a new
source needs a licence that CC BY-SA 4.0 can carry.

## Dependencies

The Nix package pins the hash of the vendored Go modules. After changing
`go.mod` or `go.sum`, set `vendorHash` in `nix/package.nix` to `lib.fakeHash`,
run `nix build`, and paste in the hash it reports.

## Releasing

Tag `vX.Y.Z`. The release workflow runs the tests and GoReleaser, which builds
static Linux binaries for amd64 and arm64 and publishes the tarballs and
`checksums.txt`. `goreleaser release --snapshot --clean` does a dry run locally.
