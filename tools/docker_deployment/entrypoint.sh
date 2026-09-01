#!/bin/sh
set -e

# bind-mounted dirs are created by the docker daemon as root on first run;
# reclaim them for the app user before dropping privileges
mkdir -p /app/data /app/themes /app/storage /app/logs /app/backups
chown knov:knov /app/data /app/themes /app/storage /app/logs /app/backups

exec su-exec knov ./knov
