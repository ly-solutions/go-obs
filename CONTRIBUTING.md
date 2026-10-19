# Contributing

Thanks for helping. Bug reports, fixes and new router integrations are all welcome.

## Before you start

- For a bug, open an issue with a minimal program that reproduces it, or go straight to a pull request with a failing
  test.
- For a new feature or integration, open an issue first so we can agree on the shape. The library deliberately keeps
  a small API and few dependencies, and metric names and labels are a contract: changing them breaks dashboards and
  alerts, so such changes need a strong reason and a major version.

## Development

You need the Go version named in `go.mod` or newer. Before sending a change, run what CI runs:

```sh
gofmt -l .                 # must print nothing
go vet ./...
go test -race -count=1 ./...
golangci-lint run          # v2, configured in .golangci.yml
govulncheck ./...
```

- Every behavior change comes with a test. Prefer tests that drive a real `net/http` server or router over mocks.
- Public API gets a doc comment, and anything a user is expected to call gets a runnable `Example` in an
  `example_test.go`, so it shows up on pkg.go.dev.
- Router integrations live in their own package (`muxobs`, `fiberobs`) so that users of plain `net/http` never compile
  a router they do not use.
- Comments say why, not what. Keep names neutral; tests use stand-ins such as `example.com`.

## Pull requests

- One logical change per pull request, with a message in the conventional form (`feat(fiberobs): …`,
  `fix: …`, `docs: …`).
- Add a line to the `Unreleased` section of `CHANGELOG.md` for anything a user would notice.
- CI must be green: tests with the race detector on the minimum and the latest Go, golangci-lint, govulncheck.

## License

By contributing you agree that your contribution is licensed under the Apache License 2.0 (see `LICENSE`).
