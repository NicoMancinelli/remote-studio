package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"remote-studio/pkg/config"
	"remote-studio/pkg/session"
	"remote-studio/pkg/status"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ClientConn struct {
	ws *websocket.Conn
	mu sync.Mutex
}

type Daemon struct {
	conn       *dbus.Conn
	props      *prop.Properties
	activeIPs  []string
	prevUsers  int
	clients    map[*ClientConn]bool
	clientsMu  sync.RWMutex
	statusJSON string
}

func getTailscaleIP() string {
	out, err := exec.Command("tailscale", "ip", "-4").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func getLanIP() string {
	out, err := exec.Command("hostname", "-I").Output()
	if err == nil {
		fields := strings.Fields(string(out))
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return "127.0.0.1"
}

func getSessionInfo() (bool, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, ""
	}
	sessionFile := filepath.Join(home, ".config", "remote-studio", "session.state")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		return false, ""
	}
	file, err := os.Open(sessionFile)
	if err != nil {
		return true, ""
	}
	defer file.Close()

	data, _ := os.ReadFile(sessionFile)
	lines := strings.Split(string(data), "\n")
	profile := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "profile=") {
			profile = strings.TrimPrefix(line, "profile=")
		}
	}
	return true, profile
}

func FindConfigDir() string {
	dir, err := os.Getwd()
	if err == nil {
		for {
			p := filepath.Join(dir, "config")
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				return p
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	execPath, err := os.Executable()
	if err == nil {
		p := filepath.Join(filepath.Dir(execPath), "config")
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return "/usr/share/remote-studio"
}

func (d *Daemon) updateStatus() {
	cur := status.GetCurrentMode()
	temp := status.GetCpuTemp()
	statusDir := status.ResolveStatusDir()
	ping := status.GetPingCached(statusDir)
	if ping == "" {
		ping = "…"
	}
	users, connType, ips := status.GetActiveUsersAndConnection()
	ram := status.GetRamUsage()
	warnCount, warnText := status.GetWarningSummary()
	netSpeed := status.GetNetSpeed()

	tailscaleIP := getTailscaleIP()
	lanIP := getLanIP()
	combinedIP := lanIP
	rustdeskDirect := lanIP + ":21118"
	if tailscaleIP != "" {
		combinedIP = tailscaleIP + "/" + lanIP
		rustdeskDirect = tailscaleIP + ":21118"
	}

	res := status.GetCurrentResolution()
	rdCodec := status.GetRustdeskCodec(users)
	pipeCodec := rdCodec
	if pipeCodec == "" {
		pipeCodec = "none"
	}

	d.activeIPs = ips

	// Update legacy text status file
	linePath := filepath.Join(statusDir, "status")
	lineContent := fmt.Sprintf("%s | %s | %s | %d | %s | %d | %s | %s | %s | %s | %s | %s | %s",
		cur, temp, ping, users, ram, warnCount, warnText, netSpeed, combinedIP, connType, res, rustdeskDirect, pipeCodec)
	_ = os.WriteFile(linePath, []byte(lineContent+"\n"), 0644)

	// Build legacy JSON
	type WarningJSON struct {
		Count   int    `json:"count"`
		Summary string `json:"summary"`
	}
	type LegacyStatusJSON struct {
		Mode          string      `json:"mode"`
		Temperature   string      `json:"temperature"`
		Latency       string      `json:"latency"`
		Users         int         `json:"users"`
		Ram           string      `json:"ram"`
		Warnings      WarningJSON `json:"warnings"`
		Network       string      `json:"network"`
		IP            string      `json:"ip"`
		Connection    string      `json:"connection"`
		Resolution    string      `json:"resolution"`
		DirectAddress string      `json:"direct_address"`
		Codec         string      `json:"codec"`
		StatusFile    string      `json:"status_file"`
		ActiveIPs     []string    `json:"active_ips"`
	}

	s := LegacyStatusJSON{
		Mode:          cur,
		Temperature:   temp,
		Latency:       ping,
		Users:         users,
		Ram:           ram,
		Warnings:      WarningJSON{Count: warnCount, Summary: warnText},
		Network:       netSpeed,
		IP:            combinedIP,
		Connection:    connType,
		Resolution:    res,
		DirectAddress: rustdeskDirect,
		Codec:         rdCodec,
		StatusFile:    linePath,
		ActiveIPs:     ips,
	}

	bytes, err := json.Marshal(s)
	if err == nil {
		d.statusJSON = string(bytes)
		if d.props != nil {
			d.props.SetMust("org.remote_studio.Daemon", "Status", d.statusJSON)
		}
	}

	// Update status.json
	active, profile := getSessionInfo()
	ramVal := 0.0
	ramClean := strings.TrimSuffix(ram, "%")
	if f, err := strconv.ParseFloat(ramClean, 64); err == nil {
		ramVal = f
	}
	disp := os.Getenv("DISPLAY")
	if disp == "" {
		disp = ":0"
	}
	netStatus := "disconnected"
	if tailscaleIP != "" || users > 0 {
		netStatus = "connected"
	}
	sState := &status.SessionStatus{
		SessionActive: active,
		SessionPID:    os.Getpid(),
		Display:       disp,
		Profile:       profile,
		NetworkStatus: netStatus,
		CPUUsage:      0.0,
		MemoryUsage:   ramVal,
	}
	_ = status.WriteStatus(sState)
}

func (d *Daemon) broadcastWS() {
	d.clientsMu.RLock()
	defer d.clientsMu.RUnlock()

	message := map[string]interface{}{
		"type": "status_full",
	}
	var data interface{}
	if err := json.Unmarshal([]byte(d.statusJSON), &data); err == nil {
		message["data"] = data
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		return
	}

	for client := range d.clients {
		go func(c *ClientConn) {
			c.mu.Lock()
			defer c.mu.Unlock()
			_ = c.ws.WriteMessage(websocket.TextMessage, msgBytes)
		}(client)
	}
}

func (d *Daemon) emitDbusSignal() {
	if d.conn != nil {
		_ = d.conn.Emit("/org/remote_studio/Daemon", "org.remote_studio.Daemon.StatusChanged", d.statusJSON)
	}
}

func (d *Daemon) Refresh() *dbus.Error {
	d.pollNetwork()
	return nil
}

func (d *Daemon) StartSession(profile string) *dbus.Error {
	if err := session.SessionStart(profile); err != nil {
		return dbus.NewError("org.remote_studio.Daemon.Error", []any{err.Error()})
	}
	d.pollNetwork()
	return nil
}

func (d *Daemon) StopSession() *dbus.Error {
	if err := session.SessionStop(); err != nil {
		return dbus.NewError("org.remote_studio.Daemon.Error", []any{err.Error()})
	}
	d.pollNetwork()
	return nil
}

func (d *Daemon) pollNetwork() {
	users, _, ips := status.GetActiveUsersAndConnection()

	trusted := true
	peerOS := ""
	if users > 0 {
		trusted = false
		tsStatusOut, err := exec.Command("tailscale", "status", "--json").Output()
		if err == nil {
			var tsStatus struct {
				Peer map[string]struct {
					TailscaleIPs []string `json:"TailscaleIPs"`
					OS           string   `json:"OS"`
				} `json:"Peer"`
			}
			if err := json.Unmarshal(tsStatusOut, &tsStatus); err == nil {
				for _, ip := range ips {
					if ip == "127.0.0.1" || ip == "localhost" {
						trusted = true
						break
					}
					for _, peerInfo := range tsStatus.Peer {
						for _, tip := range peerInfo.TailscaleIPs {
							if ip == tip {
								trusted = true
								peerOS = peerInfo.OS
								break
							}
						}
						if trusted {
							break
						}
					}
				}
			}
		} else {
			if _, errTS := exec.LookPath("tailscale"); errTS != nil {
				trusted = true
			}
		}
	}

	if !trusted {
		users = 0
	}

	cfg, _, _ := config.FindAndLoadConfig()
	autoSessionVal := os.Getenv("AUTO_SESSION")
	if autoSessionVal == "" {
		autoSessionVal = cfg.GetConfigValue("AUTO_SESSION")
	}
	autoSession := (autoSessionVal == "true")

	defaultProfile := cfg.GetConfigValue("DEFAULT_PROFILE")
	if defaultProfile == "" {
		defaultProfile = "mac"
	}

	if users > 0 && d.prevUsers == 0 {
		fmt.Printf("Session connected from trusted IP. Detected OS: %s\n", peerOS)
		if autoSession {
			profile := defaultProfile
			if peerOS == "iOS" {
				profile = "ipad"
			} else if peerOS == "macOS" {
				profile = "mac"
			} else if peerOS == "windows" || peerOS == "linux" {
				profile = "fallback"
			}
			_ = session.SessionStart(profile)
		}
	} else if users == 0 && d.prevUsers > 0 {
		fmt.Println("Session disconnected.")
		if autoSession {
			_ = session.SessionStop()
		}
	}

	d.prevUsers = users
	d.updateStatus()
	d.broadcastWS()
	d.emitDbusSignal()
}

func StartDaemon() error {
	// First check port conflicts on 9998 and 9999
	l1, err1 := net.Listen("tcp", "127.0.0.1:9998")
	if err1 != nil {
		return fmt.Errorf("port conflict: 9998 is already in use")
	}
	l1.Close()

	l2, err2 := net.Listen("tcp", "127.0.0.1:9999")
	if err2 != nil {
		return fmt.Errorf("port conflict: 9999 is already in use")
	}
	l2.Close()

	// Check DBus environment variable. An unset OR empty variable is the
	// same thing for our purposes — DBus is optional in this daemon, so we
	// only proceed to connect when the user actually pointed us at a bus.
	dbusAddr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")

	var conn *dbus.Conn
	var props *prop.Properties
	var dbusErr error

	if dbusAddr != "" {
		conn, dbusErr = dbus.ConnectSessionBus()
		if dbusErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to connect to session DBus: %v\n", dbusErr)
		}
	}

	if conn != nil {
		reply, err := conn.RequestName("org.remote_studio.Daemon", dbus.NameFlagReplaceExisting)
		if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
			conn.Close()
			return fmt.Errorf("failed to request DBus name org.remote_studio.Daemon: %v", err)
		}
	}

	d := &Daemon{
		conn:    conn,
		clients: make(map[*ClientConn]bool),
	}

	if conn != nil {
		err := conn.Export(d, "/org/remote_studio/Daemon", "org.remote_studio.Daemon")
		if err != nil {
			conn.Close()
			return fmt.Errorf("failed to export DBus object: %w", err)
		}

		// Properties
		propsSpec := map[string]map[string]*prop.Prop{
			"org.remote_studio.Daemon": {
				"Status": {
					Value:    "{}",
					Writable: false,
					Emit:     prop.EmitTrue,
				},
			},
		}
		var errProp error
		props, errProp = prop.Export(conn, "/org/remote_studio/Daemon", propsSpec)
		if errProp != nil {
			conn.Close()
			return fmt.Errorf("failed to export properties: %w", errProp)
		}
		d.props = props

		// Introspection
		node := introspect.Node{
			Name: "/org/remote_studio/Daemon",
			Interfaces: []introspect.Interface{
				introspect.IntrospectData,
				prop.IntrospectData,
				{
					Name: "org.remote_studio.Daemon",
					Methods: []introspect.Method{
						{
							Name: "Refresh",
						},
						{
							Name: "StartSession",
							Args: []introspect.Arg{
								{Name: "profile", Type: "s", Direction: "in"},
							},
						},
						{
							Name: "StopSession",
						},
					},
					Signals: []introspect.Signal{
						{
							Name: "StatusChanged",
							Args: []introspect.Arg{
								{Name: "status", Type: "s"},
							},
						},
					},
				},
			},
		}
		_ = conn.Export(introspect.NewIntrospectable(&node), "/org/remote_studio/Daemon", "org.freedesktop.DBus.Introspectable")
	}

	// Trigger initial update
	d.pollNetwork()

	// Background network polling loop
	go func() {
		for {
			time.Sleep(2 * time.Second)
			d.pollNetwork()
		}
	}()

	// WebSocket server
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := &ClientConn{ws: ws}

		d.clientsMu.Lock()
		d.clients[client] = true
		d.clientsMu.Unlock()

		// Send initial status
		client.mu.Lock()
		message := map[string]interface{}{
			"type": "status_full",
		}
		var data interface{}
		if err := json.Unmarshal([]byte(d.statusJSON), &data); err == nil {
			message["data"] = data
		}
		msgBytes, _ := json.Marshal(message)
		_ = client.ws.WriteMessage(websocket.TextMessage, msgBytes)
		client.mu.Unlock()

		defer func() {
			d.clientsMu.Lock()
			delete(d.clients, client)
			d.clientsMu.Unlock()
			_ = ws.Close()
		}()

		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				break
			}
			var req struct {
				Action string  `json:"action"`
				Cmd    string  `json:"cmd"`
				Val    float64 `json:"val"`
			}
			if err := json.Unmarshal(msg, &req); err == nil {
				if req.Action == "command" {
					parts := strings.Fields(req.Cmd)
					if len(parts) > 0 {
						execPath, err := os.Executable()
						if err == nil {
							_ = exec.Command(execPath, parts...).Start()
						}
					}
				} else if req.Action == "scale" {
					_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.interface", "text-scaling-factor", fmt.Sprintf("%g", req.Val)).Run()
				}
			}
		}
	})

	go func() {
		_ = http.ListenAndServe("0.0.0.0:9998", nil)
	}()

	// HTTP Server for dashboard
	webDir := FindConfigDir()
	// Config dir parent is toplevel which contains web/dist
	toplevel := filepath.Dir(webDir)
	webDist := filepath.Join(toplevel, "web", "dist")
	if _, err := os.Stat(webDist); err != nil {
		webDist = filepath.Join(toplevel, "web")
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/", http.FileServer(http.Dir(webDist)))

	go func() {
		_ = http.ListenAndServe("0.0.0.0:9999", httpMux)
	}()

	select {}
}
