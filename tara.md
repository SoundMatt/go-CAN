# Threat Analysis and Risk Assessment (TARA)

**Module:** github.com/SoundMatt/go-CAN  
**Generated:** 2026-07-28T01:28:52Z  
**Standard:** ISO 21434 Chapter 9  

| ID | Asset | Threat | STRIDE | CWE | Vector | Likelihood | Impact | SL | Control | Residual Risk |
|---|---|---|---|---|---|---|---|---|---|---|
| TARA-001 | interop_canutils_linux_test.go | Command injection from variable input enables arbitrary command execution | E/R | CWE-78 | Network | Medium | High | 3 | Use exec.Command with fixed command and sanitised args | Low after remediation |
| TARA-002 | interop_helpers_linux_test.go | Command injection from variable input enables arbitrary command execution | E/R | CWE-78 | Network | Medium | High | 3 | Use exec.Command with fixed command and sanitised args | Low after remediation |
| TARA-003 | adapt.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-004 | adapt.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-005 | adapt.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-006 | adapt.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-007 | adapt_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-008 | main.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-009 | main.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-010 | parser.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-011 | parser.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-012 | main.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-013 | main.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-014 | transport.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-015 | transport.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-016 | transport.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-017 | transport.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-018 | transport_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-019 | transport_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-020 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-021 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-022 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-023 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-024 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-025 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-026 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-027 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-028 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-029 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-030 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-031 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-032 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-033 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-034 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-035 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-036 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-037 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-038 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-039 | pgn.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-040 | tp.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-041 | tp.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-042 | tp.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-043 | tp.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-044 | tp.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-045 | tp.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-046 | tp.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-047 | tp.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-048 | tp.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-049 | tp.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-050 | tp_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-051 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-052 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-053 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-054 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-055 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-056 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-057 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-058 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-059 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-060 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-061 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-062 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-063 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-064 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-065 | recorder.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-066 | recorder.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-067 | e2e.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-068 | e2e.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-069 | e2e_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-070 | e2e_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-071 | e2e_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-072 | bus_linux.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-073 | bus_linux.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-074 | bus_linux_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-075 | interop_canutils_linux_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-076 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-077 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-078 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-079 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-080 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-081 | client.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-082 | client_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-083 | client_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-084 | client_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-085 | client_test.go | Integer narrowing conversion causes silent data truncation | T/D | CWE-190 | Local | Low | Medium | 1 | Add range check before conversion | Low after remediation |
| TARA-086 | main_test.go | World-readable/writable file allows unauthorised data access or tampering | I/T | CWE-732 | Local | Medium | Medium | 2 | Create file with mode 0640 or stricter | Low after remediation |
| TARA-087 | main_test.go | World-readable/writable file allows unauthorised data access or tampering | I/T | CWE-732 | Local | Medium | Medium | 2 | Create file with mode 0640 or stricter | Low after remediation |
