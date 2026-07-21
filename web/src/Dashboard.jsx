import { Users, Activity, Wifi, AlertTriangle, Thermometer, MemoryStick, MapPin } from 'lucide-react';

function LatencyBars({ latency }) {
  if (!latency || latency === "N/A" || latency === "-") return null;
  const val = parseInt(latency, 10);
  if (isNaN(val) || val <= 0) return null;
  let bars, color;
  if (val <= 30)    { bars = "▂▄▆█"; color = "#00cc66"; }
  else if (val <= 60)  { bars = "▂▄▆ "; color = "#66bbff"; }
  else if (val <= 100) { bars = "▂▄  "; color = "#ffaa00"; }
  else                 { bars = "▂   "; color = "#ff6600"; }
  return <span className="latency-bars" style={{ color }}>{bars}</span>;
}

export default function Dashboard({ status }) {
  const hasUsers = status.users > 0;
  const hasWarning = status.warnings > 0;

  return (
    <>
      {/* Main Display Status */}
      <div className="glass-card display-card highlight-glow">
        <h3>Active Display Profile</h3>
        <div className="display-info">
          <div className="metric-value huge">{status.mode}</div>
          <div className="metric-label">{status.resolution}</div>
        </div>
        <div className="telemetry-bar">
          <div className="telemetry-item">
            <span className="t-label">Codec</span>
            <span className="t-value">{status.codec}</span>
          </div>
          <div className="telemetry-item">
            <span className="t-label">Network</span>
            <span className="t-value">{status.network}</span>
          </div>
          {status.direct && (
            <div className="telemetry-item">
              <span className="t-label">Direct</span>
              <span className="t-value direct-addr" title={status.direct}>{status.direct.slice(0, 18)}...</span>
            </div>
          )}
        </div>
      </div>
      
      {/* System Telemetry */}
      <div className="glass-card telemetry-card">
        <h3>Connection Telemetry</h3>
        <div className="telemetry-grid">
          <div className="t-box">
            <Users size={24} color={hasUsers ? "var(--accent-color)" : "var(--text-secondary)"} />
            <div className="t-box-info">
              <span className="t-value">{status.users}</span>
              <span className="t-label">Active Users</span>
            </div>
          </div>
          <div className="t-box">
            <Activity size={24} color="var(--accent-color)" />
            <div className="t-box-info">
              <span className="t-value">{status.latency} <LatencyBars latency={status.latency} /></span>
              <span className="t-label">Latency</span>
            </div>
          </div>
          <div className="t-box">
            <Wifi size={24} color={hasUsers ? "var(--accent-color)" : "var(--text-secondary)"} />
            <div className="t-box-info">
              <span className="t-value">{status.connectionType}</span>
              <span className="t-label">Connection Type</span>
            </div>
          </div>
          <div className="t-box">
            <AlertTriangle size={24} color={hasWarning ? "var(--warning-color)" : "var(--accent-color)"} />
            <div className="t-box-info">
              <span className="t-value" style={{ color: hasWarning ? "var(--warning-color)" : "var(--text-primary)" }}>{status.warnings}</span>
              <span className="t-label">Warnings</span>
            </div>
          </div>
        </div>
        <div className="telemetry-grid secondary">
          <div className="t-box">
            <Thermometer size={20} color="var(--text-secondary)" />
            <div className="t-box-info">
              <span className="t-value small">{status.temp}</span>
              <span className="t-label">Temp</span>
            </div>
          </div>
          <div className="t-box">
            <MemoryStick size={20} color="var(--text-secondary)" />
            <div className="t-box-info">
              <span className="t-value small">{status.ram}</span>
              <span className="t-label">RAM</span>
            </div>
          </div>
          <div className="t-box">
            <MapPin size={20} color="var(--text-secondary)" />
            <div className="t-box-info">
              <span className="t-value small">{status.warningText !== "none" ? status.warningText.slice(0, 14) + "..." : "OK"}</span>
              <span className="t-label">Health</span>
            </div>
          </div>
          {status.active_ips && status.active_ips.length > 0 && (
            <div className="t-box">
              <Users size={20} color="var(--accent-color)" />
              <div className="t-box-info">
                <span className="t-value small">{status.active_ips[0]}</span>
                <span className="t-label">Active IP</span>
              </div>
            </div>
          )}
        </div>
      </div>
    </>
  );
}
