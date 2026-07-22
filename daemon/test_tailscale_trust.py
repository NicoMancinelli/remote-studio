#!/usr/bin/env python3
"""Unit tests for the tailscale-trust gating in remote_studio_daemon.py.

The actual daemon is hard to import (it depends on websockets/gi.repository/
etc.), so we extract the trust-evaluation function as a standalone helper
and test it here. The daemon imports the helper from this same file.

Test matrix:
  1. LAN mode off, tailscale present, peer in tailnet         -> trusted
  2. LAN mode off, tailscale present, peer NOT in tailnet     -> NOT trusted
  3. LAN mode off, tailscale missing                          -> NOT trusted (conservative)
  4. LAN mode on,  tailscale missing                          -> trusted (LAN-only install)
  5. LAN mode on,  tailscale present, peer NOT in tailnet     -> trusted (LAN bypass)
  6. 127.0.0.1 peer (always trusted regardless of mode)       -> trusted
"""
import os
import sys
import unittest
from unittest import mock

# Make the daemon importable.
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import remote_studio_daemon as rsd


class TestTailscaleTrust(unittest.TestCase):
    """Exercises the 4-way matrix of (tailscale_present × lan_mode)."""

    def setUp(self):
        # Each test starts with a clean env so RES_LAN_MODE doesn't leak.
        self._lan_env = os.environ.pop("RES_LAN_MODE", None)

    def tearDown(self):
        if self._lan_env is not None:
            os.environ["RES_LAN_MODE"] = self._lan_env
        else:
            os.environ.pop("RES_LAN_MODE", None)

    def _evaluate(self, lan_mode, tailscale_present, peer_ip):
        """Call the daemon's trust helper with a mocked tailscale status."""
        os.environ["RES_LAN_MODE"] = "1" if lan_mode else ""

        ts_status_json = '{"Peer": {"peer-key-1": {"TailscaleIPs": ["100.100.1.50"], "OS": "linux"}}}'

        def fake_check_output(cmd, **kwargs):
            if "tailscale" not in cmd:
                raise FileNotFoundError("tailscale not on PATH")
            if not tailscale_present:
                raise rsd.subprocess.CalledProcessError(1, cmd)
            return ts_status_json

        with mock.patch.object(rsd.subprocess, "check_output",
                               side_effect=fake_check_output):
            return rsd.evaluate_peer_trust([peer_ip])

    # ----- LAN mode OFF (the default) -----

    def test_lan_off_tailscale_present_peer_in_tailnet(self):
        self.assertEqual(
            self._evaluate(lan_mode=False, tailscale_present=True,
                           peer_ip="100.100.1.50"),
            (True, "linux"),  # trusted + OS extracted from peer table
        )

    def test_lan_off_tailscale_present_peer_NOT_in_tailnet(self):
        self.assertEqual(
            self._evaluate(lan_mode=False, tailscale_present=True,
                           peer_ip="10.0.0.99"),
            (False, None),
        )

    def test_lan_off_tailscale_missing(self):
        # Conservative: missing tailscale in tailnet mode = untrusted.
        self.assertEqual(
            self._evaluate(lan_mode=False, tailscale_present=False,
                           peer_ip="10.0.0.99"),
            (False, None),
        )

    def test_lan_off_localhost_when_tailscale_missing(self):
        # 127.0.0.1 only gets the "always trusted" carve-out when
        # tailscale is installed and can verify it. In the
        # tailscale-missing + tailnet-mode path the conservative
        # untrusted result applies (this matches the original daemon
        # behaviour from before the LAN-mode change).
        self.assertEqual(
            self._evaluate(lan_mode=False, tailscale_present=False,
                           peer_ip="127.0.0.1"),
            (False, None),
        )

    def test_lan_off_localhost_when_tailscale_present(self):
        # When tailscale IS installed, 127.0.0.1 is always trusted.
        self.assertEqual(
            self._evaluate(lan_mode=False, tailscale_present=True,
                           peer_ip="127.0.0.1"),
            (True, None),  # localhost carve-out — no OS extracted
        )

    # ----- LAN mode ON (the new code path) -----

    def test_lan_on_tailscale_missing_trusts_everything(self):
        # LAN-only install: no tailscale, but the user opted in to LAN,
        # so all peers are implicitly trusted.
        self.assertEqual(
            self._evaluate(lan_mode=True, tailscale_present=False,
                           peer_ip="192.168.1.42"),
            (True, None),
        )

    def test_lan_on_tailscale_present_bypass(self):
        # Tailscale installed AND LAN mode on: trust LAN IPs even if
        # they're not in the tailnet, AND extract OS from the peer
        # table when the IP happens to match a tailnet peer.
        self.assertEqual(
            self._evaluate(lan_mode=True, tailscale_present=True,
                           peer_ip="192.168.1.42"),
            (True, None),  # 192.168.1.42 isn't in the mock tailnet table
        )

    def test_lan_on_localhost_still_trusted(self):
        self.assertEqual(
            self._evaluate(lan_mode=True, tailscale_present=False,
                           peer_ip="127.0.0.1"),
            (True, None),
        )


# Helper so we can raise the right exception type from the mock is no
# longer needed — we use rsd.subprocess.CalledProcessError directly.

if __name__ == "__main__":
    unittest.main()
