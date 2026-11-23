# tssh-manager

A small Linux GUI for managing `~/.ssh/config` for [tssh](https://github.com/trzsz/trzsz-ssh).
It only manages the config; connecting still happens in your terminal with `tssh <alias>`.

## What it does

- Lists hosts grouped by the `# heading` comments in the config, with search (Ctrl+F; Enter connects the first match) and filters for plaintext passwords or failed checks.
- Add, edit, rename and delete hosts with a form, or edit a block's raw text. Only the lines you change are rewritten; everything else, including comments, order and indentation, stays byte-for-byte.
- Before each save: parse with `trzsz/ssh_config` (the parser tssh uses), check with `ssh -G`, back up the old file to `~/.config/tssh-manager/backups/` (last 30 kept), then write atomically with mode 0600. Saving is refused if the file changed on disk since it was read.
- Checks: TCP connect, then `tssh -T <alias> true` with no stdin, so a prompt fails instead of hanging. Runs 8 at a time.
- Key wizard: generate an ed25519 key, install it on the server (in the background with the stored tssh password, or with `ssh-copy-id` in a terminal), verify a key-only login, then set `IdentityFile`/`IdentitiesOnly` and remove the stored password.
- Passwords (`#!! Password`, `encPassword`, TOTP secrets…) are never sent to the UI. They appear as `******`, and leaving the mask unchanged in the raw editor keeps the stored value.

## Build

Needs Go, Node/npm, GTK3 and WebKitGTK 4.1.

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
wails build -tags webkit2_41          # -> build/bin/tssh-manager
wails dev -tags webkit2_41            # live reload
go test -tags webkit2_41 ./...        # TSSHM_REAL=1 also round-trips your real ~/.ssh/config, read-only
```

## Settings

`~/.config/tssh-manager/settings.json`: terminal command (for example `ghostty -e tssh {host}`; `{host}` becomes a separate argument and no shell is involved), tssh binary, config path and key directory.

## Layout

- `internal/sshconf`: line-level parser/editor and the file store (backup, atomic write, conflict check)
- `internal/checker`: `ssh -G` resolution, TCP and tssh login checks
- `internal/settings`: preferences and terminal detection
- `app.go`: methods bound to the frontend
- `frontend/`: Svelte 5 UI
