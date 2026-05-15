# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in convocate, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, please send an email to the project maintainers with:

1. A description of the vulnerability
2. Steps to reproduce the issue
3. Potential impact assessment
4. Any suggested fixes (if applicable)

## Response Timeline

- **Acknowledgment**: Within 48 hours of report
- **Initial Assessment**: Within 7 days
- **Fix and Disclosure**: Coordinated with the reporter

## Supported Versions

Only the latest release version is actively supported with security updates.

## Security Considerations

See README.md § Security Posture for the full security model. Key properties:

- **Network isolation**: Agent Containers have outbound internet only; no inbound
  connections; cannot reach Redis, OpenBao server, or Router API mTLS listener.
- **Credential scope**: Per-project fine-grained PATs and ed25519 deploy keys. No
  cluster-wide bot PAT. Credentials live in OpenBao, served via per-container Unix
  sockets with short-lived tokens.
- **Agent isolation**: Each project runs in a dedicated container enrolled in a cgroup
  slice (90% aggregate CPU/memory cap). Strict project-to-container binding.
- **mTLS everywhere**: All inter-service traffic uses mutual TLS with certificates
  from a private CA. Each service holds its own keypair.
- **Repository allowlist**: Job submissions from unauthorized repositories are rejected.
