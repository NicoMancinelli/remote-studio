import { useState, useEffect, useRef, useCallback } from 'react';
import './index.css';
import Sidebar from './Sidebar';
import Dashboard from './Dashboard';
import Controls from './Controls';
import Presets from './Presets';

const RECONNECT_DELAY = 3000;

// Simple settings view
function SettingsView() {
  return (
    <div className="glass-card">
      <h3>⚙ Remote Studio Settings</h3>
      <p style={{ color: 'var(--text-secondary)', marginBottom: '16px' }}>
        Configure via the Cinnamon Applet Settings:
      </p>
      <ol style={{ color: 'var(--text-secondary)', lineHeight: '2', paddingLeft: '20px' }}>
        <li>Right-click the Remote Studio panel icon</li>
        <li>Select <strong>Configure...</strong></li>
        <li>Adjust defaults for auto-session, notifications, colors, and more</li>
      </ol>
      <p style={{ color: 'var(--text-secondary)', marginTop: '16px', fontSize: '13px' }}>
        Settings file: <code>~/.cinnamon/configs/remote-studio@neek/settings.json</code>
      </p>
    </div>
  );
}

function App() {
  const [view, setView] = useState('dashboard');
  const [status, setStatus] = useState({
    connected: false,
    mode: "Idle",
    resolution: "No Resolution",
    codec: "N/A",
    network: "0 Kbps",
    users: 0,
    latency: "-",
    connectionType: "None",
    warnings: 0,
    warningText: "none",
    temp: "N/A",
    ram: "N/A",
    direct: null,
    active_ips: []
  });

  const wsRef = useRef(null);
  const reconnectTimerRef = useRef(null);
  const mountedRef = useRef(true);

  const connect = useCallback(() => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) return;
    if (wsRef.current) {
      try { wsRef.current.close(); } catch(e) {}
    }

    const socket = new WebSocket(`ws://${window.location.hostname}:9998`);

    socket.onopen = () => {
      if (!mountedRef.current) { socket.close(); return; }
      setStatus(prev => ({ ...prev, connected: true }));
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
    };

    socket.onclose = () => {
      wsRef.current = null;
      if (!mountedRef.current) return;
      setStatus(prev => ({ ...prev, connected: false, mode: "Offline" }));
      if (!reconnectTimerRef.current) {
        reconnectTimerRef.current = setTimeout(() => {
          reconnectTimerRef.current = null;
          if (mountedRef.current) connect();
        }, RECONNECT_DELAY);
      }
    };

    socket.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === "status_full") {
          const d = msg.data;
          const isIdle = d.mode === "None" || d.mode === "Idle";
          setStatus(prev => ({
            ...prev,
            mode: isIdle ? "Idle" : d.mode,
            resolution: isIdle ? "Ready for connection" : d.resolution,
            codec: d.codec || "N/A",
            network: d.network || "0 Kbps",
            users: d.users || 0,
            latency: d.latency || "-",
            connectionType: d.connection || "None",
            warnings: (d.warnings && d.warnings.count) || 0,
            warningText: (d.warnings && d.warnings.summary) || "none",
            temp: d.temperature || "N/A",
            ram: d.ram || "N/A",
            direct: d.direct_address || null,
            active_ips: d.active_ips || []
          }));
        } else if (msg.type === "status") {
          setStatus(prev => ({ ...prev, mode: msg.data }));
        }
      } catch (e) {
        console.error("Failed to parse websocket message", e);
      }
    };

    socket.onerror = () => {};

    wsRef.current = socket;
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    connect();
    return () => {
      mountedRef.current = false;
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
      if (wsRef.current) wsRef.current.close();
    };
  }, [connect]);

  const sendCommand = (cmd) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ action: "command", cmd }));
    } else {
      alert("Not connected to Remote Studio Daemon");
    }
  };

  return (
    <div className="app-container">
      <Sidebar activeView={view} onNavigate={setView} />
      <main className="main-content">
        <header>
          <div className="header-titles">
            <h1>Remote Studio</h1>
            <p className="subtitle">{view === 'dashboard' ? 'System Dashboard' : 'Settings'}</p>
          </div>
          <div className="header-actions">
            <div className="status-badge" id="conn-status">
              <span className={`dot ${status.connected ? 'green' : 'red'}`}></span>
              <span>{status.connected ? "Connected" : "Reconnecting..."}</span>
            </div>
          </div>
        </header>

        {view === 'settings' ? (
          <SettingsView />
        ) : (
          <>
            <div className="dashboard-grid">
              <Dashboard status={status} />
              <Controls sendCommand={sendCommand} />
            </div>

            {status.active_ips && status.active_ips.length > 0 && (
              <div className="glass-card active-sessions-card">
                <h3>👥 Active Sessions ({status.active_ips.length})</h3>
                <div className="active-ips">
                  {status.active_ips.map((ip, i) => (
                    <div key={i} className="ip-item">
                      <span className="ip-address">🔌 {ip}</span>
                      <button className="glass-btn small" onClick={() => sendCommand('session stop')}>
                        ⏹ Disconnect All
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            )}

            <Presets sendCommand={sendCommand} />
          </>
        )}
      </main>
    </div>
  );
}

export default App;