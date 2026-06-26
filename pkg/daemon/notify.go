package daemon

import (
	"fmt"
	"net"
	"os"
)

// SdNotifyReady sends the READY=1 notification to systemd, indicating
// that the daemon has finished initialization and is ready to serve.
//
// This implements the sd_notify(3) protocol:
//   - If $NOTIFY_SOCKET is not set, this is a no-op (not running under
//     systemd or Type=notify is not configured). Returns (false, nil).
//   - If $NOTIFY_SOCKET is set, sends "READY=1" to the Unix datagram
//     socket. Returns (true, nil) on success.
//
// The NOTIFY_SOCKET env var is unset after a successful notification
// to prevent child processes from accidentally re-using it.
func SdNotifyReady() (notified bool, err error) {
	return sdNotify("READY=1")
}

// SdNotifyStopping sends STOPPING=1 to systemd, indicating the daemon
// has begun its shutdown sequence.
func SdNotifyStopping() (bool, error) {
	return sdNotify("STOPPING=1")
}

// SdNotifyStatus sends a STATUS=<text> message to systemd, which appears
// in `systemctl status` output.
func SdNotifyStatus(status string) (bool, error) {
	return sdNotify("STATUS=" + status)
}

// sdNotify sends an arbitrary state string to systemd via $NOTIFY_SOCKET.
func sdNotify(state string) (bool, error) {
	socketAddr := os.Getenv("NOTIFY_SOCKET")
	if socketAddr == "" {
		// Not running under systemd notify — silently skip.
		return false, nil
	}

	// NOTIFY_SOCKET can be:
	//   /path/to/socket   — filesystem Unix socket
	//   @abstract-name    — abstract Unix socket (Linux-specific)
	//
	// For abstract sockets, the leading '@' is replaced with a NUL byte.
	if socketAddr[0] == '@' {
		socketAddr = "\x00" + socketAddr[1:]
	}

	conn, err := net.Dial("unixgram", socketAddr)
	if err != nil {
		return false, fmt.Errorf("sd_notify: failed to connect to %q: %w",
			os.Getenv("NOTIFY_SOCKET"), err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(state))
	if err != nil {
		return false, fmt.Errorf("sd_notify: failed to send %q: %w", state, err)
	}

	// Clear the variable so child processes don't re-use it.
	os.Unsetenv("NOTIFY_SOCKET")

	return true, nil
}
