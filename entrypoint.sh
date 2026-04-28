#!/bin/bash
set -e

CONVOCATE_UID="${CONVOCATE_UID:-1000}"
CONVOCATE_GID="${CONVOCATE_GID:-1000}"

# Create the convocate group if it doesn't exist
if ! getent group convocate > /dev/null 2>&1; then
    # Try to create with the requested GID, fall back to auto-assign if GID is taken
    groupadd -g "${CONVOCATE_GID}" convocate 2>/dev/null || groupadd convocate
fi

# Create the convocate user if it doesn't exist
if ! id convocate > /dev/null 2>&1; then
    # Try to create with the requested UID, fall back to auto-assign if UID is taken
    useradd -u "${CONVOCATE_UID}" -g convocate -d /home/convocate -s /bin/bash -m convocate 2>/dev/null \
        || useradd -g convocate -d /home/convocate -s /bin/bash -m convocate
fi

# Ensure the docker group exists and add convocate to it
DOCKER_SOCKET_GID=$(stat -c '%g' /var/run/docker.sock 2>/dev/null || echo "")
if [ -n "${DOCKER_SOCKET_GID}" ]; then
    if ! getent group "${DOCKER_SOCKET_GID}" > /dev/null 2>&1; then
        groupadd -g "${DOCKER_SOCKET_GID}" docker-host
    fi
    DOCKER_GROUP=$(getent group "${DOCKER_SOCKET_GID}" | cut -d: -f1)
    usermod -aG "${DOCKER_GROUP}" convocate 2>/dev/null || true
fi

# Set up sudoers for convocate user
echo "convocate ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/convocate
chmod 440 /etc/sudoers.d/convocate

# Ensure home directory exists and is owned by convocate
mkdir -p /home/convocate
chown convocate:convocate /home/convocate

# Add Go to path in profile
echo 'export PATH="${PATH}:/usr/local/go/bin"' > /etc/profile.d/go.sh

# Switch to convocate user and run claude inside tmux
export HOME=/home/convocate

# Start a detached tmux session running claude
sudo -E -u convocate -H -- tmux new-session -d -s convocate "/usr/local/bin/claude --dangerously-skip-permissions"

# Keep container alive while the tmux session exists
while sudo -u convocate tmux has-session -t convocate 2>/dev/null; do
    sleep 1
done
