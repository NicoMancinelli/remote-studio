import { LayoutDashboard, Settings } from 'lucide-react';

export default function Sidebar({ activeView, onNavigate }) {
  return (
    <aside className="sidebar glass-panel">
      <div className="logo">
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="url(#gradient)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <defs>
            <linearGradient id="gradient" x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stopColor="#00f2fe"/><stop offset="100%" stopColor="#4facfe"/>
            </linearGradient>
          </defs>
          <rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect>
          <line x1="8" y1="21" x2="16" y2="21"></line>
          <line x1="12" y1="17" x2="12" y2="21"></line>
        </svg>
      </div>
      <nav>
        <div
          className={`nav-item ${activeView === 'dashboard' ? 'active' : ''}`}
          title="Dashboard"
          onClick={() => onNavigate && onNavigate('dashboard')}
        >
          <LayoutDashboard size={24} />
        </div>
        <div
          className={`nav-item ${activeView === 'settings' ? 'active' : ''}`}
          title="Settings"
          onClick={() => onNavigate && onNavigate('settings')}
        >
          <Settings size={24} />
        </div>
      </nav>
    </aside>
  );
}