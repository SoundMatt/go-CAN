# Contributing to go-CAN

Thank you for your interest in contributing.

## Developer Certificate of Origin (DCO)

All contributions must be signed off under the
[Developer Certificate of Origin v1.1](https://developercertificate.org).

Add a `Signed-off-by` trailer to every commit:

```
git commit -s -m "feat: add awesome thing"
```

This produces:

```
feat: add awesome thing

Signed-off-by: Your Name <your@email.com>
```

If you forget to sign off, amend the commit:

```
git commit --amend -s
```

A GitHub Actions check (`DCO`) verifies every commit in a PR. PRs without
sign-offs will not be merged.

## Copyright

By contributing you agree that your contributions are licensed under the
[Mozilla Public License v2.0](LICENSE) and that copyright in go-CAN remains
with Matt Jones. The DCO sign-off transfers no copyright — it only affirms you
have the right to contribute under the existing license.

## Coding style

- `gofmt` — run `gofmt -w ./...` before pushing.
- `go vet` — must pass with zero warnings.
- Tests — new code should be accompanied by tests covering the public API.
  Run `go test -race -count=1 ./...` locally.

## Pull requests

1. Fork the repo, create a branch from `main`.
2. Make your changes with signed-off commits.
3. `go test -race -count=1 ./...` must pass (socketcan tests skip without vcan0).
4. Open a PR targeting `main`.

## Project structure

| Directory | What it contains |
|---|---|
| `can.go` | Core `Bus` interface, `Frame`, `Filter`, `ValidateFrame` |
| `virtual/` | In-process virtual CAN bus |
| `socketcan/` | Linux SocketCAN implementation |
| `dbc/` | DBC file parser and signal decoder |
| `isotp/` | ISO 15765-2 transport protocol |
| `j1939/` | SAE J1939 protocol layer |
| `safety/` | E2E protection header (CRC, sequence counter) |
| `cmd/cantool/` | CLI tool |
| `examples/quickstart/` | Docker quickstart |
| `docker/` | Dockerfile and docker-compose |

## Scope

go-CAN is a CAN bus library. Bridge packages to other protocols (DDS, MQTT,
SOME/IP, etc.) may live here as sub-packages — follow the same coding standard
and CI requirements.
