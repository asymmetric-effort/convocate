# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in claude-shell, please report it responsibly.

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

claude-shell runs containers with the following security-relevant properties:

- Session isolation via separate Docker containers
- Read-only bind mounts for shared configuration
- Per-session filesystem namespaces
- Dynamic UID/GID mapping to match host user

Users should be aware that:

- The Docker socket is mounted into containers, granting container-level Docker access
- Network access is enabled by default
- The claude CLI binary is bind-mounted from the host
