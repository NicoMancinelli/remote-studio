# Socket Activation on Linux — Status

**Last verified:** 2026-07-29 (pdi, Linux Mint 22.3, x86_64)

## TL;DR

`remote-studio.socket` and `remote-studio-socket.service` exist in
`systemd/` but **are not installed by `install.sh` and do not work
as-is on Linux/bash-shim installs.** The currently working daemon is
the direct `remote-studio.service` started unconditionally (no socket
activation).

## Why it's not installed

The socket-activated service unit references:

```
ExecStart=/usr/local/bin/res daemon --socket-activated
```

Two blockers on Linux/bash-shim installs (e.g. pdi):

1. **`--socket-activated` is a Go binary flag.** The Go daemon's
   `pkg/daemon/socket_activation.go` honors `LISTEN_FDS`/`LISTEN_PID`
   from the `sd_listen_fds(3)` protocol. The bash shim
   (`remote-studio/res.sh`) does not parse or forward this flag — the
   Python daemon (`daemon/remote_studio_daemon.py`) only reads
   `LISTEN_FDS`/`LISTEN_PID` indirectly (via Go) and falls back to
   binding ports directly. `./res daemon --socket-activated` on a
   bash shim fails with `unknown flag: --socket-activated`.

2. **The Go binary in the repo (`remote-studio/res`) is
   `Mach-O 64-bit arm64`** — built on macOS. On pdi (Linux x86_64),
   `/usr/local/bin/res` is the bash shim only. Building the Go daemon
   on pdi requires `go` (not installed) and a cross-compile to
   linux-amd64.

## What this means

- Installing `remote-studio.socket` alone is harmless: sockets idle
  on 9998/9999, no daemon runs, nothing serves. But it's also useless.
- Installing `remote-studio-socket.service` (which requires the
  socket) and pointing it at the bash shim with the
  `--socket-activated` flag will fail with the unknown-flag error
  above.
- The **working Linux setup** is the unconditional
  `remote-studio.service` (ExecStart calls the Python daemon
  directly, no socket activation). `install.sh` already installs
  this. The daemon binds 9998/9999 on its own.

## To make socket activation work on Linux

Pick one:

- **Build the Go daemon on pdi**: install Go, `go build -o res ./cmd/...`,
  replace the bash shim with the Go binary at `/usr/local/bin/res`,
  install `remote-studio.socket` + `remote-studio-socket.service`.
  (Socket-activation flag will then be honored.)
- **Extend the Python daemon** (`daemon/remote_studio_daemon.py`) to
  honor `LISTEN_FDS`/`LISTEN_PID` per `sd_listen_fds(3)`, then point
  the bash shim's `daemon` subcommand at a flag that disables its own
  bind. (Code change; ~50 lines.)
- **Keep the current setup** (direct service, no socket activation)
  and accept that the daemon runs continuously. 26MB RSS, ~0.1% CPU
  when idle — this is what currently runs on pdi.

## Verification (2026-07-29)

```
$ systemctl --user status remote-studio.service
   Active: active (running) since Wed 2026-07-29 09:56:11 EDT; 15min ago
   Main PID: 360902 (python3)

$ ss -lntpu | grep 9998
tcp   LISTEN 0  100  0.0.0.0:9998  ...  users:(("python3",pid=360902,fd=13))

$ ls /home/neek/.config/systemd/user/remote-studio.service
remote-studio.service  (the direct one, no socket unit present)
```

## See also

- `pkg/daemon/socket_activation.go` — Go-side implementation
- `systemd/remote-studio.socket` — listen directives (9998 + 9999)
- `systemd/remote-studio-socket.service` — service unit calling
  `/usr/local/bin/res daemon --socket-activated`
- `install.sh` — does NOT install the socket unit
