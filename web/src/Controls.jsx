import { SunMoon, Moon, MonitorSpeaker } from 'lucide-react';

export default function Controls({ sendCommand }) {
  return (
    <div className="glass-card controls-card">
      <h3>Performance & Toggles</h3>
      <div className="toggle-group">
        <button className="glass-btn" onClick={() => sendCommand('theme')}>
          <SunMoon size={18} />
          OLED Theme
        </button>
        <button className="glass-btn" onClick={() => sendCommand('night')}>
          <Moon size={18} />
          Night Shift
        </button>
        <button className="glass-btn" onClick={() => sendCommand('audio')}>
          <MonitorSpeaker size={18} />
          Audio Fix
        </button>
      </div>
    </div>
  );
}
