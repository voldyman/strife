#!/usr/bin/env bash
# Build strife and manage it as a systemd --user service on a remote host.
#
#   deploy/deploy.sh [deploy]   build, upload, install unit, restart (default)
#   deploy/deploy.sh status     service status
#   deploy/deploy.sh logs       follow logs
#   deploy/deploy.sh restart|stop|start
#   deploy/deploy.sh backup     snapshot the state dir on the server now
#   deploy/deploy.sh pull-backup  copy newest snapshot to ./deploy/backups (off-box copy)
#   deploy/deploy.sh check      verify the server has everything it needs
#
# Host defaults to "aura" (an ~/.ssh/config alias); override with STRIFE_HOST.
set -euo pipefail

HOST="${STRIFE_HOST:-aura}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SSH=(ssh -o BatchMode=yes "$HOST")
# systemctl --user over ssh needs the user's runtime dir.
USERCTL='XDG_RUNTIME_DIR=/run/user/$(id -u) systemctl --user'

cmd="${1:-deploy}"

case "$cmd" in
deploy)
	(cd "$ROOT" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$ROOT/deploy/strife.linux" ./cmd/strife)
	"${SSH[@]}" 'mkdir -p ~/.config/systemd/user ~/go/bin ~/.local/bin'
	# Back up first; `set -e` aborts the deploy if the snapshot fails.
	"$0" install-backup
	scp -q "$ROOT/deploy/strife.linux" "$HOST:go/bin/strife.new"
	scp -q "$ROOT/deploy/strife.service" "$HOST:.config/systemd/user/strife.service"
	rm -f "$ROOT/deploy/strife.linux"
	"${SSH[@]}" "chmod +x ~/go/bin/strife.new && mv ~/go/bin/strife.new ~/go/bin/strife \
		&& $USERCTL daemon-reload && $USERCTL enable strife && $USERCTL restart strife \
		&& sleep 3 && $USERCTL --no-pager status strife"
	;;
install-backup)
	"${SSH[@]}" 'mkdir -p ~/.config/systemd/user ~/.local/bin'
	scp -q "$ROOT/deploy/backup.sh" "$HOST:.local/bin/strife-backup"
	scp -q "$ROOT/deploy/strife-backup.service" "$ROOT/deploy/strife-backup.timer" "$HOST:.config/systemd/user/"
	"${SSH[@]}" "chmod +x ~/.local/bin/strife-backup && $USERCTL daemon-reload \
		&& $USERCTL enable --now strife-backup.timer && ~/.local/bin/strife-backup"
	;;
backup) "${SSH[@]}" '~/.local/bin/strife-backup' ;;
pull-backup)
	mkdir -p "$ROOT/deploy/backups"
	latest="$("${SSH[@]}" 'ls -1t ~/backups/strife/strife-state-*.tar.gz | head -1')"
	scp -q "$HOST:$latest" "$ROOT/deploy/backups/" && echo "saved $ROOT/deploy/backups/$(basename "$latest")"
	;;
status) "${SSH[@]}" "$USERCTL --no-pager status strife" ;;
logs) ssh "$HOST" 'journalctl --user -u strife -f -n 50' ;;
restart | stop | start) "${SSH[@]}" "$USERCTL $cmd strife" ;;
check)
	"${SSH[@]}" '
		ok() { printf "OK   %s\n" "$1"; }
		bad() { printf "MISS %s\n" "$1"; }
		export PATH=$HOME/.local/bin:$PATH
		for b in yt-dlp ffmpeg; do command -v $b >/dev/null && ok $b || bad "$b (install it; see deploy/README.md)"; done
		[ -x ~/go/bin/dca ] && ok ~/go/bin/dca || bad "~/go/bin/dca"
		[ -d ~/go/bin/Discord ] && ok "state dir ~/go/bin/Discord" || bad "state dir ~/go/bin/Discord"
		f=~/.config/strife/strife.env
		if [ -f $f ]; then
			[ "$(stat -c %a $f)" = 600 ] && ok "$f (0600)" || bad "$f must be chmod 600"
			grep -q "^DISCORD_TOKEN=" $f && grep -q "^DISCORD_OWNER_USER_ID=" $f && ok "env file has both keys" || bad "env file missing keys"
		else bad "$f"; fi
		[ -x ~/.local/bin/strife-backup ] && ok "backup script" || bad "backup script (run deploy.sh install-backup)"
		export XDG_RUNTIME_DIR=/run/user/$(id -u)
		systemctl --user is-enabled strife-backup.timer >/dev/null 2>&1 && ok "backup timer" || bad "backup timer (run deploy.sh install-backup)"
		loginctl show-user $USER -p Linger | grep -q yes && ok "linger enabled" || bad "linger off: sudo loginctl enable-linger $USER"
	'
	;;
*)
	sed -n '2,10p' "$0"
	exit 1
	;;
esac
