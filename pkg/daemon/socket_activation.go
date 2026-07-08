package daemon

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"
)

// listenFDsStart is the starting file descriptor number for socket activation.
// Per the sd_listen_fds(3) protocol, systemd passes FDs starting at 3.
const listenFDsStart = 3

// GetListeners returns two net.Listeners for the WebSocket (port 9998) and
// HTTP (port 9999) servers.
//
// If the process was started via systemd socket activation (LISTEN_FDS and
// LISTEN_PID are set), it inherits the file descriptors passed by systemd
// and wraps them as net.Listener. The FDs correspond to the ListenStream
// directives in remote-studio.socket, in order:
//
//	fd 3 → WebSocket (0.0.0.0:9998)
//	fd 4 → HTTP      (0.0.0.0:9999)
//
// If NOT socket-activated, it falls back to binding the ports directly.
func GetListeners() (wsListener, httpListener net.Listener, socketActivated bool, err error) {
	fds, activated := getListenFDs()
	if activated {
		if fds < 2 {
			return nil, nil, false, fmt.Errorf(
				"socket activation: expected at least 2 file descriptors, got %d", fds)
		}

		wsListener, err = fdToListener(listenFDsStart, "ws-socket")
		if err != nil {
			return nil, nil, false, fmt.Errorf(
				"socket activation: failed to create WebSocket listener from fd %d: %w",
				listenFDsStart, err)
		}

		httpListener, err = fdToListener(listenFDsStart+1, "http-socket")
		if err != nil {
			wsListener.Close()
			return nil, nil, false, fmt.Errorf(
				"socket activation: failed to create HTTP listener from fd %d: %w",
				listenFDsStart+1, err)
		}

		// Clear the environment variables so child processes don't
		// accidentally try to re-use them (per sd_listen_fds protocol).
		os.Unsetenv("LISTEN_FDS")
		os.Unsetenv("LISTEN_PID")

		return wsListener, httpListener, true, nil
	}

	// Not socket-activated — bind ports directly with SO_REUSEADDR so the
	// daemon can recover from TIME_WAIT after a previous instance was killed
	// (matters for the e2e suite, which kills and restarts daemons in quick
	// succession on ports 9998/9999).
	wsListener, err = listenReuseAddr("tcp", "0.0.0.0:9998")
	if err != nil {
		return nil, nil, false, fmt.Errorf("port conflict: failed to bind 0.0.0.0:9998: %w", err)
	}

	httpListener, err = listenReuseAddr("tcp", "0.0.0.0:9999")
	if err != nil {
		wsListener.Close()
		return nil, nil, false, fmt.Errorf("port conflict: failed to bind 0.0.0.0:9999: %w", err)
	}

	return wsListener, httpListener, false, nil
}

// listenReuseAddr binds a TCP listener with SO_REUSEADDR enabled. Plain
// net.Listen does not set the socket option, so a recently-killed listener
// in TIME_WAIT would block the bind. SO_REUSEADDR lets us reclaim the port
// immediately.
func listenReuseAddr(network, address string) (net.Listener, error) {
	cfg := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
		},
	}
	return cfg.Listen(nil, network, address)
}

// getListenFDs implements the sd_listen_fds(3) protocol:
//   - LISTEN_PID must match our PID
//   - LISTEN_FDS contains the number of passed file descriptors
//
// Returns (count, true) if socket-activated, or (0, false) otherwise.
func getListenFDs() (int, bool) {
	pidStr := os.Getenv("LISTEN_PID")
	fdsStr := os.Getenv("LISTEN_FDS")

	if pidStr == "" || fdsStr == "" {
		return 0, false
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, false
	}

	// LISTEN_PID must match our own PID.
	if pid != os.Getpid() {
		return 0, false
	}

	fds, err := strconv.Atoi(fdsStr)
	if err != nil || fds <= 0 {
		return 0, false
	}

	return fds, true
}

// fdToListener wraps a raw file descriptor (inherited from systemd) into a
// net.Listener. The FD is expected to be a listening TCP socket.
//
// The name parameter is used for the os.File name (useful in diagnostics).
func fdToListener(fd int, name string) (net.Listener, error) {
	// Create an *os.File from the raw fd. os.NewFile does not take
	// ownership of the fd — we set CloseOnExec below.
	f := os.NewFile(uintptr(fd), name)
	if f == nil {
		return nil, fmt.Errorf("invalid file descriptor %d", fd)
	}

	// net.FileListener duplicates the fd internally, so we close our
	// *os.File afterwards to avoid leaking an extra fd.
	ln, err := net.FileListener(f)
	f.Close()
	if err != nil {
		return nil, fmt.Errorf("FileListener for fd %d: %w", fd, err)
	}

	return ln, nil
}
