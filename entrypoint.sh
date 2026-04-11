#!/bin/bash
set -e

CLAUDE_UID="${CLAUDE_UID:-1000}"
CLAUDE_GID="${CLAUDE_GID:-1000}"

# Create the claude group if it doesn't exist
if ! getent group claude > /dev/null 2>&1; then
    # Try to create with the requested GID, fall back to auto-assign if GID is taken
    groupadd -g "${CLAUDE_GID}" claude 2>/dev/null || groupadd claude
fi

# Create the claude user if it doesn't exist
if ! id claude > /dev/null 2>&1; then
    # Try to create with the requested UID, fall back to auto-assign if UID is taken
    useradd -u "${CLAUDE_UID}" -g claude -d /home/claude -s /bin/bash -m claude 2>/dev/null \
        || useradd -g claude -d /home/claude -s /bin/bash -m claude
fi

# Ensure the docker group exists and add claude to it
DOCKER_SOCKET_GID=$(stat -c '%g' /var/run/docker.sock 2>/dev/null || echo "")
if [ -n "${DOCKER_SOCKET_GID}" ]; then
    if ! getent group "${DOCKER_SOCKET_GID}" > /dev/null 2>&1; then
        groupadd -g "${DOCKER_SOCKET_GID}" docker-host
    fi
    DOCKER_GROUP=$(getent group "${DOCKER_SOCKET_GID}" | cut -d: -f1)
    usermod -aG "${DOCKER_GROUP}" claude 2>/dev/null || true
fi

# Set up sudoers for claude user
echo "claude ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/claude
chmod 440 /etc/sudoers.d/claude

# Ensure home directory exists and is owned by claude
mkdir -p /home/claude
chown claude:claude /home/claude

# Add Go to path in profile
echo 'export PATH="${PATH}:/usr/local/go/bin"' > /etc/profile.d/go.sh

# Switch to claude user and exec claude
export HOME=/home/claude
exec sudo -E -u claude -H -- /usr/local/bin/claude
