# Security Policy

## Supported versions

go-CAN follows semantic versioning. Security fixes are applied to the latest
minor release line. Until a 1.0 release, only the most recent tagged version
(currently the `v0.x` series) receives security updates.

| Version | Supported          |
|---------|--------------------|
| latest `v0.x` | :white_check_mark: |
| older         | :x:                |

## Reporting a vulnerability

**Please do not open public GitHub issues for security vulnerabilities.**

Report privately using GitHub's [private vulnerability reporting][gh-advisory]
("Report a vulnerability" under the Security tab of the repository), or email
the maintainer at the address listed on the GitHub profile of
[@SoundMatt](https://github.com/SoundMatt).

Please include:

- the affected version / commit,
- a description of the issue and its impact,
- reproduction steps or a proof of concept,
- any suggested remediation.

[gh-advisory]: https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability

## Disclosure process and timeline

| Stage | Target |
|-------|--------|
| Acknowledge receipt | within **3 business days** |
| Initial assessment & severity (CVSS) | within **10 business days** |
| Fix or mitigation available | within **90 days** of acknowledgement (sooner for high severity) |
| Coordinated public disclosure | after a fix is released, by mutual agreement |

We follow coordinated disclosure and will credit reporters who wish to be named.

## Scope and threat model

go-CAN is a CAN protocol **library**, not a deployed ECU. The CAN bus itself
provides no authentication, integrity, or confidentiality; defending the
physical bus is the responsibility of the integrating system. What the library
does and does not defend against is documented in
[`THREAT_MODEL.md`](THREAT_MODEL.md).

In scope for this policy:

- Memory-safety or logic defects in parsing/handling untrusted input
  (candump logs, DBC files, ISO-TP/UDS/OBD-II/J1939 frames).
- Resource-exhaustion defects reachable from untrusted input.
- Vulnerable dependencies.

Out of scope (inherent to the CAN protocol — see the threat model):

- Spoofing of arbitration IDs by other bus nodes.
- Bus flooding / babbling-idiot denial of service.
- Passive bus sniffing.

## Tooling

- **Dependency scanning:** `govulncheck` runs in CI on every push and pull
  request; the machine-readable result is published as `vuln.json`.
- **Static safety/security checks:** `gofusa check` runs in CI.
- **Supply chain:** an SBOM (`sbom.json`) and build provenance
  (`provenance.json`) are generated for each release.

Incident response procedures are described in
[`INCIDENT-RESPONSE.md`](INCIDENT-RESPONSE.md).
