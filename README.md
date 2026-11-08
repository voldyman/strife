# strife

Strife is a pleb bot that plays music, among other things.

It requires a discord token and an optional owner user id to run.

```
go get github.com/voldyman/strife
go install github.com/voldyman/strife/cmd/strife

$GOPATH/bin/strife -discordtoken "Bot <auth token>" -discordowneruserid <owner user id>
```

The music plugin shells out to `./youtube-dl` and `./dca` (run from the working
directory) and `ffmpeg` (on `PATH`), so these must be available at runtime.
