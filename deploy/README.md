# Deploying strife

Production runs on `aura` (aura.tripent.net) as a **systemd user service** named
`strife`, run by user `voldyman`. Everything is driven from your machine with
`deploy/deploy.sh`; the host comes from the `aura` alias in `~/.ssh/config`
(override with `STRIFE_HOST`).

```
deploy/deploy.sh            # build linux/amd64, upload, install unit, restart
deploy/deploy.sh status
deploy/deploy.sh logs       # follow journal
deploy/deploy.sh restart    # also: stop, start
deploy/deploy.sh check      # verify server prerequisites
```

## Layout on the server

| Path | What |
| --- | --- |
| `~/go/bin/strife` | the binary (replaced atomically on deploy) |
| `~/go/bin/Discord/` | **bot state** (reminders, stats). The working dir; back it up, never delete |
| `~/go/bin/dca` | audio encoder, run as `./dca` from the working dir |
| `~/.local/bin/yt-dlp`, `ffmpeg` | must be on `PATH` (the unit sets `PATH`) |
| `~/.config/systemd/user/strife.service` | copied from `deploy/strife.service` |
| `~/.config/strife/strife.env` | **secrets**, mode 0600, never in git |

`~/launch-nalak` is the old foreground launcher; it is superseded by the unit
(it also still expects `youtube-dl`). Do not run both.

## Backups

`~/go/bin/Discord/` is the only irreplaceable thing on the server. It is
protected three ways:

- `deploy.sh` snapshots it **before every deploy** and aborts if that fails. A
  deploy only ever replaces the binary and unit files; it never touches `Discord/`.
- `strife-backup.timer` snapshots it daily (`~/backups/strife/`, newest 14 kept,
  each archive verified).
- `deploy.sh pull-backup` copies the newest snapshot to `deploy/backups/` here
  (gitignored), so a copy exists off the server. Run it now and then.

`deploy.sh backup` takes a snapshot on demand. To restore: stop the service,
extract a snapshot into `~/go/bin/`, start the service.

## Secrets

The unit is checked in, the token is not. The unit points at
`EnvironmentFile=%h/.config/strife/strife.env`, which only exists on the server:

```
DISCORD_TOKEN="Bot <token>"
DISCORD_OWNER_USER_ID=<numeric id>
```

`strife` reads these as defaults for `-discordtoken` / `-discordowneruserid`, so
the token never appears in `ps`. Create it with `umask 077` and `chmod 600`. The
older `~/.config/nalak/credentials.json` holds the same values if you need to
convert them. Never print, log or commit the token.

## One-time server setup

No passwordless sudo, so a human runs the sudo steps:

```
sudo apt install ffmpeg
sudo loginctl enable-linger voldyman     # keep user services running after logout
curl -fsSL -o ~/.local/bin/yt-dlp https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux
chmod +x ~/.local/bin/yt-dlp
```

Then create `strife.env` as above, run `deploy/deploy.sh check` until everything
says OK, then `deploy/deploy.sh`.

Keep yt-dlp current (YouTube breaks old versions): `ssh aura '~/.local/bin/yt-dlp -U'`.

## Notes for agents

- Do not stop the old bot or start two instances with the same token.
- Run `deploy/deploy.sh check` before deploying and `status`/`logs` after.
- Run `go vet ./... && go test ./...` before deploying.
- Use `ssh aura`, not `aura.tripent.net`, which fails host-key verification here.
