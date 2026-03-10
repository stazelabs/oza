# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in Oza, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, please email **security@stazelabs.com** with:

- A description of the vulnerability
- Steps to reproduce the issue
- The potential impact
- Any suggested fix (optional)

We will acknowledge receipt within 48 hours and aim to provide a fix or mitigation plan within 7 days for confirmed vulnerabilities.

## Scope

The following areas are in scope:

- The `oza` archive format parser and writer (`oza/`, `ozawrite/`)
- The CLI tools (`cmd/`)
- The MCP server (`ozamcp/`)
- The HTTP serve layer (`ozaserve/`)
- The ZIM-to-OZA converter (`zim2oza/`)

## Security Measures

- **Fuzz testing**: Core parsers are continuously fuzzed in CI to catch panics, OOMs, and other input-handling bugs.
- **Race detection**: All tests run with `-race` enabled.
- **Static analysis**: Code is checked with golangci-lint on every PR.
- **Dependency scanning**: CodeQL analysis runs on every push and pull request.
