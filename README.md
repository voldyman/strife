# strife

Strife is a pleb bot that plays music, among other things.

It requires a discord token and an optional owner user id to run.

```
go get github.com/voldyman/strife
go install github.com/voldyman/strife/cmd/strife

$GOPATH/bin/strife -discordtoken "Bot <auth token>" -discordowneruserid <owner user id>
# or set DISCORD_TOKEN and DISCORD_OWNER_USER_ID instead of the flags
```

The music plugin shells out to `yt-dlp` and `ffmpeg` (both on `PATH`) and `./dca` (run from the
working directory), so these must be available at runtime.

Production deployment: see [deploy/README.md](deploy/README.md).
