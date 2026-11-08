#!/usr/bin/env bash
# Snapshot the bot's state dir (~/go/bin/Discord). Runs on the server, from
# deploy.sh and from the daily strife-backup.timer. Keeps the newest $KEEP.
set -euo pipefail
SRC="$HOME/go/bin/Discord"
DEST="$HOME/backups/strife"
KEEP="${KEEP:-14}"

[ -d "$SRC" ] || { echo "backup: missing $SRC" >&2; exit 1; }
mkdir -p "$DEST"
umask 077
out="$DEST/strife-state-$(date +%Y%m%d-%H%M%S).tar.gz"
tar czf "$out" -C "$(dirname "$SRC")" "$(basename "$SRC")"
tar tzf "$out" >/dev/null # verify the archive reads back
ls -1t "$DEST"/strife-state-*.tar.gz | tail -n +$((KEEP + 1)) | xargs -r rm -f
echo "$out"
