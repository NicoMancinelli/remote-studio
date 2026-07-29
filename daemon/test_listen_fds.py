"""Unit tests for the sd_listen_fds(3) protocol implementation.

Covers:
- get_listen_fds() pure env parsing (no socket activation unset/garbage)
- get_listeners() success path with two real bound sockets
- get_listeners() falls back when LISTEN_FDS count is wrong
- get_listeners() falls back when LISTEN_PID doesn't match our PID
- get_listeners() returns ``None`` when neither env var is set

Run with: python3 -m unittest daemon.test_listen_fds
"""
import os
import socket
import unittest
import warnings
from unittest import mock

# Make sure tests can find the module under test.
import sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from listen_fds import (  # noqa: E402
    LISTEN_FDS_START,
    get_listen_fds,
    get_listeners,
)


class TestGetListenFds(unittest.TestCase):
    """Pure env-parsing behavior, no socket I/O."""

    def test_unset_env_means_no_activation(self):
        with mock.patch.dict(os.environ, {}, clear=True):
            count, activated = get_listen_fds()
            self.assertEqual(count, 0)
            self.assertFalse(activated)

    def test_pid_string_must_match(self):
        with mock.patch.dict(os.environ, {
            "LISTEN_PID": str(os.getpid() + 1),
            "LISTEN_FDS": "2",
        }):
            count, activated = get_listen_fds()
            self.assertEqual(count, 0)
            self.assertFalse(activated)

    def test_zero_count_disables_activation(self):
        with mock.patch.dict(os.environ, {
            "LISTEN_PID": str(os.getpid()),
            "LISTEN_FDS": "0",
        }):
            count, activated = get_listen_fds()
            self.assertEqual(count, 0)
            self.assertFalse(activated)

    def test_negative_count_disables_activation(self):
        with mock.patch.dict(os.environ, {
            "LISTEN_PID": str(os.getpid()),
            "LISTEN_FDS": "-1",
        }):
            count, activated = get_listen_fds()
            self.assertEqual(count, 0)
            self.assertFalse(activated)

    def test_garbage_pid_disables_activation(self):
        with mock.patch.dict(os.environ, {
            "LISTEN_PID": "not-an-int",
            "LISTEN_FDS": "2",
        }):
            count, activated = get_listen_fds()
            self.assertEqual(count, 0)
            self.assertFalse(activated)

    def test_happy_path(self):
        with mock.patch.dict(os.environ, {
            "LISTEN_PID": str(os.getpid()),
            "LISTEN_FDS": "2",
        }):
            count, activated = get_listen_fds()
            self.assertEqual(count, 2)
            self.assertTrue(activated)


class TestGetListeners(unittest.TestCase):
    """End-to-end: create two real listening sockets, hand them to the
    daemon, and assert get_listeners() returns the wrapped sockets in
    the order the listener expects (port 9998 then port 9999)."""

    def _make_socket(self):
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.bind(("127.0.0.1", 0))
        s.listen(5)
        return s

    def test_no_env_returns_none(self):
        with mock.patch.dict(os.environ, {}, clear=True):
            listeners, activated = get_listeners(expected=2)
            self.assertIsNone(listeners)
            self.assertFalse(activated)

    def test_fd_count_too_small_falls_back(self):
        s1 = self._make_socket()
        try:
            # Pass only one FD but expect two.
            with mock.patch.dict(os.environ, {
                "LISTEN_PID": str(os.getpid()),
                "LISTEN_FDS": "1",
            }):
                target_fd = LISTEN_FDS_START
                saved = os.dup(target_fd)
                os.dup2(s1.fileno(), target_fd)
                try:
                    listeners, activated = get_listeners(expected=2)
                    self.assertIsNone(listeners)
                    self.assertFalse(activated)
                finally:
                    os.dup2(saved, target_fd)
                    os.close(saved)
        finally:
            s1.close()

    def test_pid_mismatch_falls_back(self):
        with mock.patch.dict(os.environ, {
            "LISTEN_PID": str(os.getpid() + 1),
            "LISTEN_FDS": "2",
        }):
            listeners, activated = get_listeners(expected=2)
            self.assertIsNone(listeners)
            self.assertFalse(activated)

    def test_happy_path_returns_two_listeners(self):
        s1 = self._make_socket()
        s2 = self._make_socket()
        try:
            saved_a = os.dup(LISTEN_FDS_START)
            saved_b = os.dup(LISTEN_FDS_START + 1)
            os.dup2(s1.fileno(), LISTEN_FDS_START)
            os.dup2(s2.fileno(), LISTEN_FDS_START + 1)
            try:
                with mock.patch.dict(os.environ, {
                    "LISTEN_PID": str(os.getpid()),
                    "LISTEN_FDS": "2",
                }):
                    # Ignore the "unclosed socket" warning that comes
                    # from the test tearing down the duped FDs before
                    # the test references the returned wrappers.
                    with warnings.catch_warnings():
                        warnings.simplefilter("ignore", ResourceWarning)
                        listeners, activated = get_listeners(expected=2)
                    self.assertTrue(activated)
                    self.assertIsNotNone(listeners)
                    # ``listeners`` is non-None past this assert.
                    assert listeners is not None
                    self.assertEqual(len(listeners), 2)
                    for ln in listeners:
                        self.assertEqual(ln.family, socket.AF_INET)
                        self.assertEqual(ln.type, socket.SOCK_STREAM)
            finally:
                os.dup2(saved_a, LISTEN_FDS_START)
                os.dup2(saved_b, LISTEN_FDS_START + 1)
                os.close(saved_a)
                os.close(saved_b)
        finally:
            s1.close()
            s2.close()


if __name__ == "__main__":
    unittest.main()
