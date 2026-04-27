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
  loops and explicit work-list patterns. See `internal/session/
  copyDir` for the canonical iterative pattern.
- **ed25519-only SSH keys.** Every key the project generates must
  be ed25519. No RSA, no ECDSA.

## Getting Started

1. Fork the repository
2. Clone your fork locally
3. Create a feature branch: `git checkout -b feature/my-feature`
4. Make your changes
5. Run linters: `make lint`
6. Run tests: `make test`
7. Commit your changes with a clear commit message
8. Push to your fork and submit a pull request

## Development Prerequisites

- Go 1.26+
- Docker
- yamllint
- jsonlint (via npm: `npm install -g jsonlint`)

## Building

```bash
make build
```

## Testing

```bash
# Run all tests
make test

# Run linters
make lint
```

## Pull Request Guidelines

- Keep changes focused and atomic
- Include tests for new functionality
- Ensure all tests pass before submitting
- Update documentation as needed
- Follow existing code style and conventions

## Reporting Issues

- Use GitHub Issues to report bugs
- Include steps to reproduce the issue
- Include expected and actual behavior
- Include Go version, Docker version, and OS information

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
