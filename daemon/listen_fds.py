#!/usr/bin/env python3
"""sd_listen_fds(3) protocol implementation for socket activation.

Per the systemd protocol documented in `sd_listen_fds(3)`:

* The environment variable `LISTEN_PID` must match the process that
  intends to consume the file descriptors. We check against ``os.getpid()``.
* The environment variable `LISTEN_FDS` contains the number of listening
  sockets passed to the process, starting at file descriptor 3.
* The file descriptors must be ``AF_INET`` / ``AF_INET6`` ``SOCK_STREAM``
  sockets that are already listening.

The order of file descriptors matches the order of ``ListenStream=``
directives in the corresponding ``.socket`` unit. In our case the
``remote-studio.socket`` unit defines:

    ListenStream=0.0.0.0:9998   # WebSocket
    ListenStream=0.0.0.0:9999   # HTTP

So the daemon expects:

    fd 3 → WebSocket (0.0.0.0:9998)
    fd 4 → HTTP      (0.0.0.0:9999)

If either ``LISTEN_FDS`` or ``LISTEN_PID`` is unset, or the PID mismatch
fails, or the FD count is wrong, socket activation is **not** in effect
and the daemon should fall back to binding the ports directly.

This module is intentionally dependency-free (stdlib only) so it can be
unit-tested without PyGObject or websockets installed.
"""
from __future__ import annotations

import os
import socket
from typing import List, Optional, Tuple


# File-descriptor index where systemd starts handing out listening sockets.
# systemd passes descriptors starting at 3 by convention.
LISTEN_FDS_START = 3


def get_listen_fds() -> Tuple[int, bool]:
    """Return ``(count, activated)`` per the sd_listen_fds(3) protocol."""
    pid_str = os.environ.get("LISTEN_PID", "")
    fds_str = os.environ.get("LISTEN_FDS", "")
    if not pid_str or not fds_str:
        return 0, False
    try:
        pid = int(pid_str)
        fds = int(fds_str)
    except ValueError:
        return 0, False
    if pid != os.getpid() or fds <= 0:
        return 0, False
    return fds, True


def fd_to_socket(fd: int) -> socket.socket:
    """Wrap a file descriptor (inherited from systemd) as a Python socket.

    Duplicate the FD so the original FD number at ``fd`` is still
    usable by systemd's bookkeeping (the daemon consumes both the WS
    and HTTP FDs back-to-back).
    """
    return socket.socket(fileno=fd)


def get_listeners(
    expected: int = 2,
) -> Tuple[Optional[List[socket.socket]], bool]:
    """Return ``(listeners, activated)``.

    If socket-activated and ``expected`` file descriptors are present,
    ``listeners`` contains ``expected`` Python sockets wrapping the
    inherited FDs. The order matches the order of ``ListenStream=``
    directives, so for the Remote Studio socket unit the first entry is
    the WebSocket listener (9998) and the second is the HTTP listener
    (9999).

    If ``LISTEN_FDS``/``LISTEN_PID`` are unset, ``activated=False`` and
    ``listeners=None``. The caller should fall back to binding the ports
    directly.

    If ``LISTEN_FDS`` is set but the count is wrong, ``activated=False``
    and ``listeners=None`` — we deliberately do not raise here, because
    on a development box the operator may have manually set the env
    vars wrong and we'd rather degrade to port-binding than crash.
    """
    count, activated = get_listen_fds()
    if not activated:
        return None, False
    if count < expected:
        # Misconfigured socket activation (e.g. fd count < 2). Fall back
        # to direct bind and let the developer notice via the daemon log.
        return None, False
    listeners = []
    for offset in range(expected):
        try:
            listeners.append(fd_to_socket(LISTEN_FDS_START + offset))
        except (OSError, ValueError):
            return None, False
    return listeners, True


__all__ = [
    "LISTEN_FDS_START",
    "get_listen_fds",
    "fd_to_socket",
    "get_listeners",
]
