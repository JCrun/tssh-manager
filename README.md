# tssh-manager

A small GUI for managing `~/.ssh/config` for [tssh](https://github.com/trzsz/trzsz-ssh), on Linux, macOS and Windows.
It only manages the config; connecting still happens in your terminal with `tssh <alias>`.
Optional end-to-end encrypted sync keeps the config (and, if you want, your keys) the same on all your machines.

## What it does

- Lists hosts grouped by the `# heading` comments in the config, with search (Ctrl+F; Enter connects the first match) and filters for plaintext passwords or failed checks.
- Add, edit, rename and delete hosts with a form, or edit a block's raw text. Only the lines you change are rewritten; everything else, including comments, order and indentation, stays byte-for-byte.
- Before each save: parse with `trzsz/ssh_config` (the parser tssh uses), check with `ssh -G`, back up the old file to `~/.config/tssh-manager/backups/` (last 30 kept), then write atomically with mode 0600. Saving is refused if the file changed on disk since it was read.
- Checks: TCP connect, then `tssh -T <alias> true` with no stdin, so a prompt fails instead of hanging. Runs 8 at a time.
- Key wizard: generate an ed25519 key, install it on the server (in the background with the stored tssh password, or with `ssh-copy-id` in a terminal), verify a key-only login, then set `IdentityFile`/`IdentitiesOnly` and remove the stored password.
- Plaintext `#!! Password`: reveal (auto-hides after 20s), copy (clipboard cleared after 30s), set, change or delete per host. Every saved password is read back with tssh's own parser and must match exactly. `#` is not allowed, because tssh treats it as a comment.
- `#!! encPassword` is flagged as weak: tssh encrypts it with a fixed key that is public in its source, so it is no safer than plaintext.
- Passwords (`#!! Password`, `encPassword`, TOTP secrets…) are never sent to the UI unless you click reveal. They appear as `******`, and leaving the mask unchanged in the raw editor keeps the stored value.

## Multi-device sync

Sync is off until you set it up under **同步** (or with `tssh-sync setup`). You need two things on every device:

- **A remote**, which is one of:
  - **a folder** that another tool already syncs: Syncthing, iCloud Drive, OneDrive, Dropbox, Google Drive, or a NAS mount. Each device enters its own local path.
  - **WebDAV**: 坚果云 (`https://dav.jianguoyun.com/dav/<folder>/`, use an app password), Nextcloud, Synology, and so on.
- **The same sync passphrase.** The remote only ever holds `tssh-manager.sync.age`, an [age](https://age-encryption.org) file encrypted with that passphrase (scrypt). You can open it by hand with `age -d`. If you forget the passphrase, the remote copy cannot be recovered. Your local config is not affected.

How a sync works:

- Each device remembers the snapshot it last synced. A sync does a three-way merge between that snapshot, your local config and the remote, one `Host` block at a time. If two devices edited different hosts, both edits are kept. Only a host changed differently on both sides (including edit on one side and delete on the other) becomes a conflict. Nothing is written until you choose 本机 or 远端 for each conflict.
- Section headings and comments stay attached to the host below them. Your local formatting is kept.
- The merged config goes through the same save path as an edit: it is parsed, checked with `ssh -G`, the old file is backed up, and the new one is written atomically. The upload fails if another device uploaded in the meantime (WebDAV ETag / content hash). The sync then merges again.
- **Auto sync** (on by default) runs at startup and a few seconds after each save.
- **Sync keys** (off by default) also syncs the files in the key directory, encrypted. A key deleted on another device is moved to `backups/keys/` instead of being deleted.
- Folder remotes warn you about conflict copies made by Syncthing or Dropbox.
- The passphrase and WebDAV password are stored in `sync.json` (mode 0600) in the settings directory. That file gets the same protection as the plaintext `#!! Password` lines already in `~/.ssh/config`. To change the passphrase, delete the remote file, sync once from one device with the new passphrase, then enter it on the other devices.
- If a device has been offline for a long time or you are unsure of its state, **重置同步状态** makes the next sync list every differing host as a conflict instead of picking a side.

### tssh-sync (command line)

`tssh-sync` uses the same settings and the same remote file, so it also works on headless servers, in Termux on Android, or from cron:

```sh
tssh-sync setup folder ~/Sync/tssh            # or: setup webdav https://dav.example.com/ssh/ --user me [--keys]
tssh-sync                                     # sync once; exit code 3 = conflicts (printed with secrets masked)
tssh-sync run --prefer remote                 # resolve all conflicts with one side
tssh-sync status | reset | off
```

The passphrase and WebDAV password are read from the terminal, or from `TSSHM_SYNC_PASSPHRASE` / `TSSHM_WEBDAV_PASSWORD`.

## Platforms

| | Linux | macOS | Windows |
|---|---|---|---|
| Runtime | GTK 3 + WebKitGTK 4.1 | built in | WebView2 (built into Windows 10/11) |
| Settings | `~/.config/tssh-manager` | `~/Library/Application Support/tssh-manager` | `%AppData%\tssh-manager` |
| Default terminal | first installed of ghostty, kitty, wezterm, alacritty, foot, konsole, gnome-terminal, ptyxis, xfce4-terminal, xterm | Ghostty / WezTerm / kitty / Alacritty app, else Terminal.app | Windows Terminal (`wt.exe`), else a new console window |
| Key install without stored password | `ssh-copy-id` in the terminal | `ssh-copy-id` | tssh in the terminal appends the key (there is no `ssh-copy-id`) |

`XDG_CONFIG_HOME` overrides the settings location on every platform. On macOS, Homebrew's bin directories are added to `PATH`, because apps started from Finder do not inherit your shell's `PATH`. The tssh path is resolved before it is handed to the terminal.

## Build

Linux needs Go, Node/npm, GTK3 and WebKitGTK 4.1. macOS needs the Xcode command line tools. Windows needs only Go and Node.

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
wails build -tags webkit2_41          # -> build/bin/tssh-manager
wails dev -tags webkit2_41            # live reload
go test -tags webkit2_41 ./...        # TSSHM_REAL=1 also round-trips your real ~/.ssh/config, read-only
wails build -platform darwin/universal   # on macOS -> build/bin/tssh-manager.app
wails build -platform windows/amd64      # on Windows, or cross-compiled from Linux -> build/bin/tssh-manager.exe
go build -o build/bin/tssh-sync ./cmd/tssh-sync   # CLI, pure Go, any GOOS/GOARCH
```

## Install

Prebuilt builds are on the [Releases](https://github.com/JCrun/tssh-manager/releases) page:

- **Linux x86_64**: unpack and run `./install.sh`. Needs GTK 3 and WebKitGTK 4.1 (`webkit2gtk-4.1` on Arch, `libwebkit2gtk-4.1-0` on Debian/Ubuntu).
- **macOS** (universal): move `tssh-manager.app` to Applications. It is not signed, so run `xattr -dr com.apple.quarantine /Applications/tssh-manager.app` once, or right-click and choose Open.
- **Windows x64**: unzip and run `tssh-manager.exe`.
- **tssh-sync** on its own for Linux, macOS, Windows, FreeBSD and Android (Termux), amd64 and arm64.

Each desktop package also contains `tssh-sync`.

From source:

```sh
scripts/install.sh              # Linux: build, then install tssh-manager and tssh-sync to ~/.local/bin with an app-menu entry and icon
scripts/install.sh --no-build   # install the existing build/bin/tssh-manager
scripts/install.sh --uninstall  # remove binary, menu entry and icon (keeps settings and backups)
```

## Settings

`settings.json` in the settings directory: terminal command (for example `ghostty -e tssh {host}`; `{host}` becomes a separate argument and no shell is involved. On macOS, `@terminal-app tssh {host}` opens Terminal.app), tssh binary, config path and key directory.

## Releases

Pushing a `v*` tag runs `.github/workflows/build.yml`. It builds and tests on Linux, macOS and Windows runners, cross-compiles `tssh-sync`, and publishes everything to a GitHub Release with sha256 files. Pushes to main and PRs run the same checks without releasing.

## Layout

- `internal/sshconf`: line-level parser/editor and the file store (backup, atomic write, conflict check)
- `internal/checker`: `ssh -G` resolution, TCP and tssh login checks
- `internal/settings`: preferences and per-OS terminal detection
- `internal/syncer`: encrypted sync: per-host three-way merge, folder and WebDAV remotes
- `internal/proc`: process launching for Linux, macOS and Windows (no console windows flashing on Windows)
- `cmd/tssh-sync`: command line sync
- `app.go`: methods bound to the frontend
- `frontend/`: Svelte 5 UI
