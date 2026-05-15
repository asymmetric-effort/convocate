# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- Complete rewrite for v0.2.0 MVP: Router API + Redis + OpenBao control plane,
  per-host Dispatch + OpenBao Agent + Secrets Broker, per-project long-lived
  Agent Containers, Web-UI-driven project lifecycle.
- Replaced v1 SSH-based multi-host orchestration with RESTful HTTPS + mTLS
  service communication.
- All non-agent container images now use distroless base images.
- Agent Container images use ubuntu:latest.

### Removed

- v1 `convocate`, `convocate-host`, `convocate-agent` binaries.
- v1 SSH peering, rsyslog TLS, TUI menu system.
- v1 `site/` documentation site (replaced by Web UI).
