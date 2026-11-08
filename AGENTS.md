# strife

Go Discord bot (plugins in `*plugin/`, entry point `cmd/strife`).

- Build/test: `go build ./... && go vet ./... && go test ./...`
- Deploy and operate the production server (`aura`): see `deploy/README.md`;
  use `deploy/deploy.sh`.
- Secrets live only on the server (`~/.config/strife/strife.env`); never commit or print them.
- The music plugin needs `yt-dlp` and `ffmpeg` on `PATH` and `./dca` in the working directory.
