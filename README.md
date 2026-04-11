# claude-shell

[![CI](https://github.com/sam-caldwell/claude-shell/actions/workflows/ci.yml/badge.svg)](https://github.com/sam-caldwell/claude-shell/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/sam-caldwell/claude-shell/badges/.badges/coverage.json)](https://github.com/sam-caldwell/claude-shell/actions/workflows/ci.yml)

A Go wrapper around the Claude CLI that provides isolated, containerized sessions for concurrent use.

## Overview

claude-shell launches Claude CLI inside Docker containers, providing:

- **Session isolation**: Each session runs in its own container with a dedicated filesystem
- **Concurrent sessions**: Run multiple sessions simultaneously with strong isolation guarantees
- **Session persistence**: Sessions persist across restarts via UUID-named directories
- **Shared configuration**: Claude settings, credentials, and plugins are shared read-only across sessions
- **Developer tools**: Each container comes pre-loaded with common development tools

## Prerequisites

- Linux (tested on Ubuntu 22.04+)
- Docker
- Go 1.26+ (for building from source)

## Installation

### From Source

```bash
git clone https://github.com/asymmetric-effort/claude-shell.git
cd claude-shell
make build
sudo make install
```

### Post-Install Setup

```bash
sudo claude-shell install
```

This will:
- Build the Docker container image
- Create the `claude` user (if not exists)
- Configure the skeleton directory
- Install the Claude CLI (if not present)

## Usage

```bash
claude-shell
```

This presents an interactive menu:

```
claude-shell - Session Manager

  # | Name              | Session ID                           | Created    | Last Accessed
  1 | + New Session      |                                      |            |
  2 | api-refactor       | a1b2c3d4-e5f6-7890-abcd-ef1234567890 | 2026-04-10 | 2026-04-11
  3 | bug-investigation  | f9e8d7c6-b5a4-3210-fedc-ba9876543210 | 2026-04-08 | 2026-04-09
  D | Delete a session   |                                      |            |

Select option:
```

## Architecture

### Session Isolation

Each session gets:
- A unique UUIDv4 identifier
- Its own home directory at `/home/claude/<uuid>/`
- A dedicated Docker container named `claude-session-<uuid>`
- Per-session Claude history, backups, and session data

### Shared Resources (Read-Only)

- Claude CLI binary (`/usr/local/bin/claude`)
- SSH keys (`~/.ssh/`)
- Git configuration (`~/.gitconfig`)
- Claude settings and credentials (`~/.claude/` as `~/.claude-shared/`)

### Container Environment

Based on Ubuntu 24.04 with:
- build-essential, cmake, pkg-config
- Python 3, pip
- Node.js, npm
- Go
- git, curl, wget, jq, ripgrep, tmux, vim, nano
- SSH client, CA certificates

## Development

```bash
make build    # Build the binary
make test     # Run all tests
make lint     # Run linters
make clean    # Clean build artifacts
```

## License

MIT License - see [LICENSE.txt](LICENSE.txt)
