# Contributing to stdocs

Thanks for your interest in `stdocs`. This document covers the day-to-day
mechanics of contributing: how to run the tests, how to file issues.

## Development setup

```bash
git clone https://github.com/FumingPower3925/stdocs
cd stdocs
go test -race -count=1 ./...
golangci-lint run ./...
```

Requirements:

- Go 1.24 or later; CI runs the suite on every stable patch release
  at or above that floor, and the lint/format jobs use the latest
  stable.
- `golangci-lint` v2.12.2 (matches CI; `brew install golangci-lint` on macOS).

## Running the tests

The main module has no third-party runtime or test dependencies. Two
test runs cover the project:

```bash
# Unit + race tests + fuzz the pattern parser
go test -race -count=1 ./...
go test -fuzz=^FuzzParsePattern$ -fuzztime=10s ./internal/pattern/

# YAML round-trip — this is a SEPARATE go module so that gopkg.in/yaml.v3
# never appears in the main module's dep graph. It is not run by the
# plain `go test ./...` above (CI runs it inside every per-version
# compatibility job).
cd internal/spec/yaml/roundtrip_test && go test ./...
```

## Filing issues

Open an issue at <https://github.com/FumingPower3925/stdocs/issues>. Bug
reports should include a minimal reproduction; feature requests should
explain the use case, not just the proposed API.

## Pull requests

All changes land through pull requests — `main` is protected, for
maintainers too. CI must be green (the `CI OK` check) before merge.

- Keep changes focused; one concern per PR.
- Update the README, CHANGELOG, and godoc comments as needed.
- All CI jobs must pass before review: `Test`, `Lint`, `Coverage`,
  and the per-version compatibility matrix (`Go X.Y.Z`), which runs
  the suite on every supported Go release.

## Releasing

Releases are cut by the project maintainers from `main`. The release
process moves the `[Unreleased]` entries in `CHANGELOG.md` into a
new version section, tags the commit, and publishes a GitHub Release. Contributors do not need to cut
releases.
