#!/bin/sh
# ===== Backend Entrypoint =====
# Create /data subdirs as root (for mounted volumes) then drop to app user

set -e

# Ensure /data directories exist with correct ownership
mkdir -p /data/uploads /data/reports
chown -R app:app /data

# Drop to app user and execute the command
echo "[entrypoint] Switching to 'app' user and executing: $*"
exec su-exec app "$@"
