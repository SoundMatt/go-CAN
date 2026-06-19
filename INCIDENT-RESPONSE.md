# Cybersecurity Incident Response Plan

This plan covers how security incidents affecting the go-CAN library are
detected, triaged, and resolved. It satisfies IEC 62443-4-2 CR 6.2.1 (incident
response) for the component. Vulnerability *reporting* is described in
[`SECURITY.md`](SECURITY.md); this document covers what happens after a report
is received or an incident is detected.

## Roles

| Role | Responsibility |
|------|----------------|
| Maintainer ([@SoundMatt](https://github.com/SoundMatt)) | Incident lead: triage, fix, coordination, disclosure. |
| Reporter | External or internal party who discovered the issue. |
| Downstream integrators | Notified via the GitHub Security Advisory and release notes. |

## Severity classification

Severity is assigned with CVSS v3.1. Response targets:

| Severity | CVSS | Triage start | Fix target |
|----------|------|--------------|-----------|
| Critical | 9.0–10.0 | same business day | ≤ 14 days |
| High | 7.0–8.9 | ≤ 3 business days | ≤ 30 days |
| Medium | 4.0–6.9 | ≤ 5 business days | ≤ 90 days |
| Low | 0.1–3.9 | best effort | next scheduled release |

## Response process

1. **Detection / intake.** Sources: private vulnerability report, a failing
   `govulncheck` run in CI, a `gofusa check` finding, or a maintainer
   observation. Every intake is recorded as a (private) tracking issue.
2. **Triage.** Confirm reproducibility, determine affected versions, assign
   CVSS severity and a CVE/GHSA identifier where warranted.
3. **Containment.** If a vulnerable dependency is implicated, pin or remove it.
   If the defect is in go-CAN, identify the smallest safe change and add a
   regression test (`//fusa:test`) that fails before the fix.
4. **Eradication & fix.** Implement the fix on a private branch, run the full
   test suite, `gofusa check`, and `govulncheck`.
5. **Recovery / release.** Tag a patch release; the release workflow regenerates
   SBOM, provenance, and the safety/compliance artifacts.
6. **Disclosure.** Publish a GitHub Security Advisory, update `SECURITY.md`'s
   supported-versions table if needed, and credit the reporter (with consent).
7. **Post-incident review.** Record root cause and any process improvement in
   the advisory; add or update a requirement in `.fusa-reqs.json` if the
   incident revealed a missing security property.

## Communication

- **Private** until a fix is available: GitHub private advisory + direct contact
  with the reporter.
- **Public** after release: GitHub Security Advisory, release notes, and (for
  Critical/High) a note in the README changelog.

## Contact

Report and escalate via GitHub private vulnerability reporting on the
[go-CAN repository](https://github.com/SoundMatt/go-CAN), or the maintainer
contact listed on [@SoundMatt](https://github.com/SoundMatt).
