# Software Verification Plan (SVP)
## go-CAN — ISO 26262 ASIL-B / IEC 61508 SIL 2

**Document ID:** SVP-001
**Version:** 1.0
**Status:** Draft
**Author:** Matt Jones (matt@jellybaby.com)
**Standards:** ISO 26262:2018 Part 6 §10, IEC 61508-3:2010 §7.9

This SVP is the verification-focused companion to `SAFETY_PLAN.md` (the
Software Safety Plan): the SSP defines the overall safety lifecycle and
responsibilities, this document defines *how* go-CAN's safety requirements
are actually verified — methods, tools, independence, coverage objectives,
and pass/fail criteria — in accordance with ISO 26262:2018 Part 6 (Software
unit design and implementation / verification) and IEC 61508-3:2010 §7.9
(Software module and integration testing).

---

## 1. Purpose

This plan describes the verification activities applied to go-CAN
(`github.com/SoundMatt/go-CAN`), a Safety Element Out Of Context (SEOOC)
CAN bus library targeting ASIL-B (ISO 26262) / SIL 2 (IEC 61508). It
defines:

- The methods and tools used to verify each safety requirement
  (`REQ-*`, tracked in `.fusa-reqs.json` and traced via `//fusa:req` /
  `//fusa:test` source annotations)
- Independence requirements for verification activities
- Structural coverage objectives and how they are measured
- Pass/fail criteria that gate a release
- How verification is re-run and re-evidenced on every change (regression)

**Out of scope:** hardware verification, system-level (integrator)
verification, and DO-178C-style DAL-based objectives — go-CAN is a
ground-vehicle / industrial software element, not airborne software; see
`SAFETY_PLAN.md` §1 for the full scope statement.

---

## 2. Verification Methods

| Method | Tool | Applicability | What it verifies |
|---|---|---|---|
| Static analysis (coding standard + defect patterns) | `gofusa check` | ASIL-B / SIL 2 | `CODING_STANDARD.md` rules; gate on ERROR-severity findings |
| Race detection | `go test -race` | ASIL-B / SIL 2 | Data races in concurrent paths (virtual bus, socketcan, subscriptions) |
| Requirement-based testing | `go test` + `gofusa verify` | ASIL-B / SIL 2 | Every `REQ-*` has a passing `//fusa:test`-annotated test; evidence bundled to `.fusa-evidence.json` |
| Requirement traceability | `gofusa trace -req-coverage 100` | ASIL-B / SIL 2 | 100% of requirements traced AND tested; 100% function-annotation density |
| Structural coverage | `gofusa coverage` (from `go test -coverprofile`) | ASIL-B / SIL 2 | Statement coverage per package, see §4 |
| Fuzz testing | `go test -fuzz` | ASIL-B / SIL 2 | `virtual.Bus.Send` and `dbc.Parse` against malformed/adversarial input |
| Live interop testing | `can-utils` (`cangen`/`candump`) + two-process self-interop | ASIL-B / SIL 2 | Real `vcan0` wire-level frame fidelity against an independent (non-go-CAN) SocketCAN peer; see `ROADMAP.md` Milestone 19 |
| Dependency vulnerability scan | `govulncheck`, `gofusa vuln` | ASIL-B / SIL 2 | Known CVEs in the (minimal) dependency graph reachable from called code |
| Cybersecurity analysis | `gofusa cyber` | ASIL-B / SIL 2 | ISO 21434-aligned static findings (see `THREAT_MODEL.md`, `tara.json`) |
| Tool qualification | `gofusa qualify` | ASIL-B / SIL 2 | go-FuSa's own qualification suite, evidencing the verification tool itself is fit for purpose (ISO 26262-8 §11) |

All of the above run in CI (`.github/workflows/ci.yml`) on every push and
pull request against `main`; none may be skipped by a contributor.

---

## 3. Independence

Per ISO 26262-6 §10.4.4 / IEC 61508-3 Table A.6, independent verification
becomes mandatory as ASIL/SIL increases. go-CAN targets ASIL-B / SIL 2,
where independence between the person who authored the code under test and
the person who verifies it is recommended but not mandated to the degree
required at ASIL-C/D or SIL 3/4.

Current assignment, consistent with `SAFETY_PLAN.md` §8:

- **Developer:** Matt Jones — authors production code and its accompanying
  requirement-based tests.
- **Verifier (tool-level):** automated, via CI — `gofusa check`, `gofusa
  verify`, `gofusa trace`, `gofusa cyber`, `gofusa vuln`, `gofusa qualify`,
  and `go test -race` all run independently of the author on every change,
  and a pull request cannot merge unless every gate passes. This provides
  *tool independence* (the verification tool and its configuration are not
  authored per-change) even where *person* independence is not yet in place.
- **Person-level independent review:** not yet formally assigned. For any
  future ASIL-C/D or SIL 3/4 integration, or before this project accepts
  external safety-relevant contributions at scale, a reviewer distinct
  from the primary author must review each release's safety-relevant
  changes before tagging. Until then, this is a known, explicit gap
  in this document rather than an implicit assumption — see §6.

---

## 4. Coverage Objectives

| ASIL / SIL | Statement coverage | Branch/decision coverage | Notes |
|---|---|---|---|
| ASIL-B / SIL 2 | ≥ 80% per package (project target; enforced by review, not yet a hard CI gate) | Not separately measured | `go test -coverprofile` + `gofusa coverage` |
| ASIL-C/D / SIL 3/4 (future) | 100% | 100% (MC/DC not applicable to Go's control-flow model without a dedicated tool) | Out of scope until a higher-ASIL integration is undertaken |

Coverage is measured by `go test -race -count=1 -coverprofile=coverage.out
./...` and summarised by `gofusa coverage -format json -output
coverage-report.json coverage.out`, regenerated on every tagged release.
Current per-package coverage is recorded in `coverage-report.json`
(committed evidence, regenerated by `release.yml`); as of this writing
every package exercised by CI sits at or above 85%, with `mock` at 100%.

---

## 5. Pass/Fail Criteria

A change (pull request) is verification-complete only if, in CI:

1. `go vet ./...` reports no issues.
2. `go test -race -count=1 ./...` passes with zero failures (socketcan
   tests skip, not fail, when `vcan0` is unavailable on non-Linux runners
   — see `.github/workflows/ci.yml`'s `test-socketcan` job for the Linux
   case, which does exercise real `vcan0`).
3. `gofusa check ./...` reports zero ERROR-severity findings.
4. `gofusa trace -req-coverage 100` reports 100% requirement traceability
   (every `REQ-*` both traced to source and covered by a passing test) and
   100% function-annotation density.
5. `gofusa cyber` reports zero ERROR-severity findings.
6. `gofusa vuln` and `govulncheck` report zero exploitable vulnerabilities
   in code actually reachable from go-CAN.
7. `gofusa qualify` — all tool-qualification cases pass.
8. `relay conform --strict` and `relay interop --protocol CAN --strict`
   both pass (RELAY spec §12/§20 conformance — see `SAFETY_PLAN.md` §5).

A tagged release additionally requires the full `release.yml` safety-
evidence regeneration pipeline (SBOM, provenance, TARA, FMEA, safety case,
compliance gap reports) to complete without a masked failure — see
`.github/workflows/release.yml`.

---

## 6. Regression Strategy

- Every push and pull request against `main` re-runs the full verification
  suite in §2 via GitHub Actions (`.github/workflows/ci.yml`); nothing is
  cached across semantically-different code.
- Every tagged release (`vX.Y.Z`) re-runs the safety-evidence pipeline in
  `.github/workflows/release.yml` and commits the regenerated artifacts
  (`check-report.json`, `cyber-report.json`, `tara.json`, `fmea.json`,
  `coverage-report.json`, `safety-case.*`, `sbom.json`, `provenance.json`,
  compliance gap reports) back to `main`, so the evidence in the repository
  always matches the tagged tool version and source tree.
- Requirement traceability is re-derived from source annotations on every
  run, not maintained by hand, so a requirement can never silently lose
  its test coverage without CI catching it (`gofusa trace -req-coverage
  100` fails the build otherwise).

---

## 7. Approvals

| Role | Name | Date |
|---|---|---|
| Verification Lead | Matt Jones | 2026-07-29 |
| Independent Reviewer (person-level) | *unassigned — see §3* | — |

This document is a living plan: update it whenever verification methods,
tools, coverage targets, or independence assignments change, and treat an
unfilled Approvals row as a genuine open item, not a template artifact.
