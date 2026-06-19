# go-CAN Threat Model

This document records the cybersecurity threat analysis for the go-CAN
library, aligned with ISO/SAE 21434 (road-vehicle cybersecurity) and
IEC 62443-4-2 (component security requirements). It is the companion to the
functional-safety analysis in [`SAFETY_PLAN.md`](SAFETY_PLAN.md) and
[`fmea.json`](fmea.json).

go-CAN is a **library and CLI**, not a deployed ECU. It provides the CAN data
link, ISO-TP transport, and diagnostic (UDS/OBD-II/J1939) building blocks. The
integrator who embeds go-CAN owns the system-level security concept; this model
scopes what the library itself defends against versus what it delegates.

## Item definition and trust boundaries

```
   ┌────────────────────────────────────────────────────────┐
   │ Integrator application (out of scope for this library)  │
   │   ┌──────────────────────────────────────────────────┐ │
   │   │ go-CAN public API                                 │ │
   │   │   Bus.Send / Bus.Subscribe   ← TRUST BOUNDARY (A) │ │
   │   │   isotp / uds / obdii / j1939 / dbc / recorder    │ │
   │   └──────────────────────────────────────────────────┘ │
   └───────────────▲──────────────────────────▲─────────────┘
                   │                          │
        TRUST BOUNDARY (B)          TRUST BOUNDARY (C)
        SocketCAN / driver          The physical CAN bus
                                    (untrusted, shared, no native auth)
```

- **(A) API boundary** — values supplied by the embedding application. Defended
  by input validation.
- **(B) Driver boundary** — frames handed to/from the OS SocketCAN layer.
- **(C) Bus boundary** — the physical multi-master CAN network. CAN has **no
  native authentication, confidentiality, or integrity**. Any node can transmit
  any arbitration ID. This is an inherent property of the protocol, not a defect
  of this library.

## Threats (STRIDE)

| # | STRIDE | Threat | Disposition |
|---|--------|--------|-------------|
| T1 | Tampering | Malformed/oversized frame injected through the public API corrupts internal state | **Mitigated** — `ValidateFrame` rejects out-of-range IDs, bad DLC, and inconsistent RTR/FD/BRS before transmit ([REQ-SEC-001]). |
| T2 | Tampering | Forged ISO-TP Consecutive Frames spliced into a multi-frame reassembly | **Mitigated** — sequence-number check aborts reassembly on mismatch ([REQ-SEC-002]). |
| T3 | Denial of Service | Hostile peer sends an unbounded/oversized ISO-TP transfer to exhaust memory | **Mitigated** — 4095-byte protocol cap on send and receive; bounded subscriber channels with explicit back-pressure ([REQ-SEC-003]). |
| T4 | Tampering | Crafted DBC physical value wraps/truncates onto unintended bits | **Mitigated** — encoder clamps to the signal's representable range ([REQ-SEC-004]). |
| T5 | Spoofing | A rogue bus node transmits frames with a legitimate node's arbitration ID | **Delegated** — not solvable at the data-link layer. The integrator must apply message authentication (e.g. AUTOSAR SecOC / ISO 21434 freshness+MAC) above go-CAN. Documented here so it is not silently assumed away. |
| T6 | Denial of Service | Bus flooding / babbling-idiot node monopolises arbitration | **Delegated** — requires a hardware/transceiver-level mitigation (bus guardian, rate limiting). go-CAN surfaces drop/error metrics ([`MetricsProvider`](can_optional.go)) so the integrator can detect it. |
| T7 | Information Disclosure | Passive bus sniffing reveals payloads | **Delegated** — CAN is a broadcast medium. Confidentiality, if required, is an application-layer concern. |
| T8 | Repudiation | No record of frames sent/received | **Partially mitigated** — the `recorder` package provides candump-format capture for forensic logging; integrity of the log is the integrator's responsibility. |
| T9 | Elevation of Privilege | UDS routines (e.g. ECUReset, WriteDataByIdentifier) invoked without authorisation | **Delegated** — go-CAN is the diagnostic *client*; SecurityAccess (SID 0x27) enforcement is an ECU-side responsibility. The client surfaces negative responses (NRC 0x33 securityAccessDenied) rather than masking them. |

## Residual risk and integrator responsibilities

Because trust boundary (C) is fundamentally untrusted, an integrator deploying
go-CAN in a safety- or security-relevant function **must**:

1. Add message authentication (SecOC or equivalent) where frame authenticity matters.
2. Provide a bus-off / babbling-idiot recovery and rate-limiting strategy.
3. Treat the `recorder` replay capability as a powerful tool — restrict who can
   replay logs onto a live bus.
4. Enforce UDS SecurityAccess on the ECU side before honouring privileged services.

## Verification

The mitigated threats (T1–T4) are traced to requirements `REQ-SEC-001`..`004`
in [`.fusa-reqs.json`](.fusa-reqs.json) and verified by `//fusa:test`-annotated
unit tests. Dependency vulnerabilities are scanned in CI with `govulncheck`
(see [`SECURITY.md`](SECURITY.md)).
