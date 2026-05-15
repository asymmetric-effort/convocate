# Contributing to convocate

Thank you for your interest in contributing to convocate.

## Coding standards

**Authoritative reference:** <http://coding-standards.asymmetric-effort.com/>

That site is the source of truth for code style across every
asymmetric-effort project. Read it before submitting a change. The
project-specific guidance below augments it but does not override it
— if anything here conflicts with the standards site, the standards
site wins. Two project-level rules worth flagging up front:

- **No recursion in Go.** Go has no tail-call optimization; we use
  loops and explicit work-list patterns.
- **ed25519-only SSH keys.** Every key the project generates must
  be ed25519. No RSA, no ECDSA.
- **Minimal third-party dependencies.** Prefer writing our own over
  pulling in third-party libraries; adopt a dependency only when
  explicitly approved. Any third-party dependency must be MIT or
  MIT-compatible (Apache-2.0, BSD, MPL-2.0 for unmodified
  redistribution).

## Getting Started

1. Fork the repository
2. Clone your fork locally
3. Create a feature branch: `git checkout -b feature/my-feature`
4. Make your changes
5. Run linters: `make lint`
6. Run tests: `make test`
7. Commit your changes using Conventional Commits format
8. Push to your fork and submit a pull request

## Development Prerequisites

- Go 1.26+
- Docker 25+, Docker Compose
- Node.js 24+ (for Web UI)
- yamllint

## Git Hooks

Install pre-commit and pre-push hooks before making changes:

```bash
make hooks
```

This installs:
- **pre-commit**: gofmt check, go vet, golangci-lint (fast mode)
- **pre-push**: full test suite with 98% coverage threshold enforcement

## Building

```bash
make build        # build all binaries
make images       # build OCI container images
make local/start  # bring up the full dev ecosystem
```

## Testing

```bash
make test         # unit + integration tests
make test-e2e     # end-to-end tests (requires make local/start)
make lint         # go vet + yaml lint + govulncheck
```

## Pull Request Guidelines

- Keep changes focused and atomic
- Include tests for new functionality (98% coverage target)
- Ensure all tests pass before submitting
- Update documentation as needed
- Follow existing code style and conventions
- Use Conventional Commits format for commit messages

## Reporting Issues

- Use GitHub Issues to report bugs
- Include steps to reproduce the issue
- Include expected and actual behavior
- Include Go version, Docker version, and OS information

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
