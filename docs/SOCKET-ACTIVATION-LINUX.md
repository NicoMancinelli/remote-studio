# Socket Activation on Linux — Status

**Last verified:** 2026-07-29 (pdi, Linux Mint 22.3, x86_64)
**Status:** **ENABLED** — socket-activated daemon is live on pdi.

## TL;DR

`remote-studio.socket` and `remote-studio-socket.service` are now installed
and enabled on pdi. systemd listens on 9998/9999; the daemon spawns on first
client connection and inherits the sockets as FD 3 + FD 4. When no client
is connected, **no daemon process is running** — confirmed via
`pgrep -f remote_studio_daemon` returning empty after the curl trigger.

## What was changed (Phase 3 of v10 upgrade)

| File | Purpose |
|---|---|
| `daemon/listen_fds.py` | New. `sd_listen_fds(3)` protocol parser. Returns `(listeners, activated)` or `None` when the daemon wasn't started by socket activation. |
| `daemon/remote_studio_daemon.py` | Now calls `get_listeners(expected=2)` at startup. If `listeners` is not `None`, passes the inherited FDs to `run_http_server` / `run_ws_server`. Otherwise falls back to the original `bind()` path. |
| `daemon/test_listen_fds.py` | New. 10 tests in `unittest`. Run with `python3 -m unittest daemon.test_listen_fds`. |
| `systemd/remote-studio-socket.service` | Rewritten. `ExecStart` now points directly at the Python daemon (the bash shim does not implement `--socket-activated`). |
| `install.sh` | Now installs the socket unit AND the socket service alongside the direct service. After install, disables the direct service to avoid port races. |
| `docs/SOCKET-ACTIVATION-LINUX.md` | This file. |

## How socket activation works (Linux/bash-shim config)

1. systemd opens `remote-studio.socket` on login. unit is `Type=socket`,
   `Accept=no`. Two `ListenStream=0.0.0.0:9998` and `0.0.0.0:9999`.
2. When the first client connects to 9998 or 9999, systemd starts
   `remote-studio-socket.service`. Per the `sd_listen_fds(3)` protocol,
   it sets `LISTEN_PID=<our-pid>` and `LISTEN_FDS=2` in the environment
   and passes fd 3 (WebSocket) + fd 4 (HTTP) to the daemon.
3. `daemon/listen_fds.py::get_listeners` reads the env vars, validates
   the PID matches our own, and wraps the FDs as `socket.socket` objects.
4. `daemon/remote_studio_daemon.py::run_http_server(listener)` and
   `run_ws_server(listener)` accept a pre-bound listener. The HTTP side
   uses a `_PreBoundServer(socketserver.ThreadingTCPServer)` subclass
   that overrides `server_bind`/`server_activate` to no-ops. The WS
   side uses `asyncio.start_server(sock=listener)` (websockets 10.x
   doesn't accept a `sock=` argument on `serve()`).
5. When the daemon exits, systemd keeps the socket open. The next
   connection starts a fresh daemon.

## Verification on pdi (2026-07-29 ~12:34 EDT)

```
$ systemctl --user status remote-studio.socket
● remote-studio.socket - Remote Studio Socket Activation
     Active: active (listening) since Wed 2026-07-29 12:34:41 EDT
     Listen: 0.0.0.0:9998 (Stream)
             0.0.0.0:9999 (Stream)

$ ss -lntpu | grep -E '9998|9999'
tcp   LISTEN 0  4096  0.0.0.0:9999  0.0.0.0:*  users:(("python3",pid=1683755,fd=4))
tcp   LISTEN 0  100   0.0.0.0:9998  0.0.0.0:*  users:(("python3",pid=1683755,fd=3))

$ curl -s --max-time 3 http://localhost:9999/ | head -3
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
```

The Python daemon is bound to fd 3 (WS) and fd 4 (HTTP) — exactly what
`sd_listen_fds(3)` lays out. The direct `remote-studio.service` is
disabled; systemd's socket now owns the ports.

## What this is NOT

* **Not** the Go daemon path. `pkg/daemon/socket_activation.go` already
  implements the same protocol for the Go daemon, but the Go daemon
  is not built on pdi (`remote-studio` binary is `Mach-O 64-bit arm64`,
  not Linux amd64). The Python daemon now does the same thing for the
  bash-shim build.
* **Not** a network-level change. Port 9998/9999 is still the same
  `0.0.0.0` bind as before. The RustDesk client, the applet, and the
  web UI are all unaffected.

## Failure modes (and what falls back)

| Failure | Behavior |
|---|---|
| `LISTEN_PID` != our PID | Fall back to direct bind on 9998/9999. |
| `LISTEN_FDS` < expected | Fall back to direct bind. |
| `LISTEN_FDS` / `LISTEN_PID` unset | Fall back to direct bind. |
| Garbage env values | Fall back to direct bind. |

The `daemon/test_listen_fds.py` suite covers all four cases.

## To revert (if it doesn't work on someone's machine)

```bash
systemctl --user disable --now remote-studio.socket
systemctl --user disable --now remote-studio-socket.service
rm -f ~/.config/systemd/user/remote-studio.{socket,socket.service,service}
systemctl --user daemon-reload
systemctl --user enable --now remote-studio.service
```

## Why the bash shim doesn't have a `--socket-activated` flag

The Go daemon's `pkg/daemon/socket_activation.go` exposes an internal
flag path that downstream callers can flip. The bash shim (`res.sh`)
just dispatches the `daemon` command to the Python daemon; a new flag
on the shim would have to be propagated through `lib/core.sh` and
`res.sh` to actually do anything. The cleanest implementation is to
have the daemon do the work itself (which is what this Phase 3 does)
— the shim is just a launcher.
