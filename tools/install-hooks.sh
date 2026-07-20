#!/bin/bash
# Install git hooks for the convocate project
HOOKS_DIR=$(git rev-parse --git-dir)/hooks
ln -sf ../../git-hooks/pre-commit "$HOOKS_DIR/pre-commit"
echo "Git hooks installed."
