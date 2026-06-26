import { ArrowRight } from 'lucide-react';

export default function Presets({ sendCommand }) {
  return (
    <>
      <h2 className="section-title">Quick Presets</h2>
      <div className="presets-grid">
        <div className="preset-card glass-card" onClick={() => sendCommand('ipad')}>
          <div className="preset-icon">📱</div>
          <div className="preset-info">
            <h4>iPad Pro</h4>
            <p className="specs">2064 x 2752 • Auto-Scaled</p>
          </div>
          <ArrowRight className="action-arrow" />
        </div>
        
        <div className="preset-card glass-card" onClick={() => sendCommand('mac')}>
          <div className="preset-icon">💻</div>
          <div className="preset-info">
            <h4>Mac / Retina</h4>
            <p className="specs">2560 x 1664 • Auto-Scaled</p>
          </div>
          <ArrowRight className="action-arrow" />
        </div>

        <div className="preset-card glass-card" onClick={() => sendCommand('reset')}>
          <div className="preset-icon">🖥️</div>
          <div className="preset-info">
            <h4>Standard 1080p</h4>
            <p className="specs">1920 x 1080 • Standard</p>
          </div>
          <ArrowRight className="action-arrow" />
        </div>
      </div>
    </>
  );
}
