# Claude Session

## Environment

This is an isolated Claude session running inside a Docker container.

## Available Tools

- **Languages**: Go, Python 3, Node.js
- **Build Tools**: build-essential, cmake, pkg-config
- **Utilities**: git, curl, wget, jq, ripgrep, tmux, vim, nano
- **Network**: SSH client, full network access

## Session Isolation

This session has its own home directory and filesystem namespace.
Changes made here will not affect other sessions.

## Shared Configuration

Claude settings, credentials, and plugins are shared read-only from the host
via the `~/.claude-shared/` directory.
