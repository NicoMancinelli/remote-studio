const Applet    = imports.ui.applet;
const PopupMenu = imports.ui.popupMenu;
const Settings  = imports.ui.settings;
const Util      = imports.misc.util;
const GLib      = imports.gi.GLib;
const Gio       = imports.gi.Gio;
const St        = imports.gi.St;

const DBusProxy = Gio.DBusProxy.makeProxyWrapper(
    '<node><interface name="org.remote_studio.Daemon"><property name="Status" type="s" access="read"/><method name="Refresh"/><signal name="StatusChanged"><arg name="status" type="s"/></signal></interface></node>'
);

const RES_CMD     = "/usr/local/bin/res";
const RUNTIME_DIR = GLib.getenv("XDG_RUNTIME_DIR") || GLib.get_user_runtime_dir();
function _fallbackUid() {
    let envUid = GLib.getenv("UID") || GLib.getenv("EUID");
    if (envUid) return envUid;
    let runtimeMatch = (RUNTIME_DIR || "").match(/\/(\d+)$/);
    return runtimeMatch ? runtimeMatch[1] : GLib.get_user_name();
}

const FALLBACK_UID = _fallbackUid();
const STATUS_DIR  = (RUNTIME_DIR && GLib.file_test(RUNTIME_DIR, GLib.FileTest.IS_DIR))
    ? RUNTIME_DIR + "/remote-studio"
    : "/tmp/remote-studio-" + FALLBACK_UID;
const STATUS_FILE = STATUS_DIR + "/status";
const STATE_FILE  = GLib.get_home_dir() + "/.res_state";

// Resolve profiles.conf from the res symlink; fall back to .deb install path
function _safeReadLink(path) {
    try {
        return GLib.file_read_link(path, null);
    } catch(e) {
        return null;
    }
}

const _resLink    = _safeReadLink(RES_CMD);
const _repoRoot   = _resLink ? GLib.path_get_dirname(_resLink) : null;
const _repoProfs  = _repoRoot ? _repoRoot + "/config/profiles.conf" : null;
const PROFILES_FILE = (_repoProfs && GLib.file_test(_repoProfs, GLib.FileTest.EXISTS))
    ? _repoProfs
    : "/usr/share/remote-studio/profiles.conf";
const USER_PROFILES_FILE = GLib.get_home_dir() + "/.config/remote-studio/profiles.conf";
const FAVORITES_FILE     = GLib.get_home_dir() + "/.config/remote-studio/favorites.json";
const SESSION_START_FILE = STATUS_DIR + "/.session_start";

const TERMINAL_CANDIDATES = [
    'x-terminal-emulator', 'gnome-terminal', 'xfce4-terminal',
    'lxterminal', 'sakura', 'terminator', 'xterm'
];

const MODE_ICONS = {
    "MacBook Air 13":  "computer-symbolic",
    "MacBook Air 15":  "computer-symbolic",
    'iPad Pro 11"':    "tablet-symbolic",
    "iPad Pro 13":     "tablet-symbolic",
    "iPhone Landscape":"phone-symbolic",
    "iPhone Portrait": "phone-symbolic",
    "Reset":           "view-refresh-symbolic",
};

const MAX_PANEL_LABEL = 28;
const CONFIRM_TIMEOUT_MS = 3000;

// ── Applet ────────────────────────────────────────────────────────────────────

function MyApplet(metadata, orientation, panel_height, instance_id) {
    this._init(metadata, orientation, panel_height, instance_id);
}

MyApplet.prototype = {
    __proto__: Applet.TextIconApplet.prototype,

    _init: function(metadata, orientation, panel_height, instance_id) {
        Applet.TextIconApplet.prototype._init.call(this, orientation, panel_height, instance_id);

        this.set_applet_icon_name("video-display-symbolic");
        this.set_applet_tooltip("Remote Studio Dashboard");
        this.set_applet_label("Res");

        // ── Cinnamon Settings (properly wired) ─────────────────────────────
        this.settings = new Settings.AppletSettings(this, metadata.uuid, instance_id);
        this.settings.bindProperty(Settings.BindingDirection.IN,
            "default_profile", "defaultProfile", () => {
                this._buildMenu();
            });
        this.settings.bindProperty(Settings.BindingDirection.BIDIRECTIONAL,
            "notify_enabled", "_notifyEnabled", (val) => {
                this._notifyEnabled = val;
            });
        this.settings.bindProperty(Settings.BindingDirection.IN,
            "auto_session", "autoSession", null);
        this.settings.bindProperty(Settings.BindingDirection.IN,
            "label_color_enabled", "_labelColorEnabled", (val) => {
                // Re-apply color with new setting
                this._scheduleUpdate();
            });
        this.settings.bindProperty(Settings.BindingDirection.IN,
            "uptime_display", "_uptimeDisplay", () => {
                this._scheduleUpdate();
            });
        this.settings.bindProperty(Settings.BindingDirection.IN,
            "show_latency_bars", "_showLatencyBars", null);
        this.settings.bindProperty(Settings.BindingDirection.IN,
            "middle_click_action", "_middleClickAction", null);
        this.settings.bindProperty(Settings.BindingDirection.IN,
            "dbus_reconnect", "_dbusReconnect", null);
        this.settings.bindProperty(Settings.BindingDirection.IN,
            "confirm_destructive", "_confirmDestructive", null);
        this.settings.bindProperty(Settings.BindingDirection.IN,
            "auto_session_profile", "_autoSessionProfile", null);

        this.menuManager    = new PopupMenu.PopupMenuManager(this);
        this.menu           = new Applet.AppletPopupMenu(this, orientation);
        this.menuManager.addMenu(this.menu);

        this.lastUserCount  = 0;
        this._timeout       = null;
        this._fileMonitor   = null;
        this._profilesMonitors = [];
        this._notifyEnabled = true;  // will be overwritten by settings bind
        this._refreshInFlight = false;
        this._clickGuard    = false;
        this._lastClick     = 0;
        this._terminal      = null;
        this._profileCache  = null;
        this._profileCacheTime = 0;
        this._sessionStart  = null;
        this._activeIPs     = [];
        this._lastStatus    = null;
        this._confirmItems  = {};  // { itemKey: { timeoutId, callback } }
        this._dbusWatchId   = undefined;
        this._dbusRetryTimeout = null;
        this._statusFileMonitor = null;

        // Remember last used profile for "last" option
        this._lastUsedProfile = null;

        // Instance-level group collapse state (avoid sharing across instances)
        this._groupOpen = { active_sessions: true, presets: true, performance: false, rustdesk: false, system: false };

        // Load favorites
        this._loadFavorites();
        // Load session start time
        this._loadSessionStart();

        this._scheduleUpdate();
        this._setupDBus();
        this._buildMenu();

        // Apply initial label color
        this._applyLabelColor("#aaaaaa");
    },

    // ── Settings helpers ───────────────────────────────────────────────────

    _getDefaultProfile: function() {
        let profile = this.defaultProfile || "";
        // Handle "last" option — use last used profile
        if (profile === "last") {
            let lastUsed = this._getLastUsedProfile();
            if (lastUsed) return lastUsed;
            return "mac"; // fallback
        }
        if (profile) return profile;
        try {
            let f = GLib.get_home_dir() + "/.config/remote-studio/remote-studio.conf";
            if (GLib.file_test(f, GLib.FileTest.EXISTS)) {
                let [ok, buf] = GLib.file_get_contents(f);
                if (ok) { let m = buf.toString().match(/^DEFAULT_PROFILE=(.+)$/m); if (m) return m[1].trim(); }
            }
        } catch(e) {}
        return "mac";
    },

    // ── Favorites management ─────────────────────────────────────────────

    _loadFavorites: function() {
        this._favorites = [];
        try {
            if (GLib.file_test(FAVORITES_FILE, GLib.FileTest.EXISTS)) {
                let [ok, buf] = GLib.file_get_contents(FAVORITES_FILE);
                if (ok) {
                    this._favorites = JSON.parse(buf.toString());
                    if (!Array.isArray(this._favorites)) this._favorites = [];
                }
            }
        } catch(e) {
            this._favorites = [];
        }
    },

    _saveFavorites: function() {
        try {
            let dir = GLib.get_home_dir() + "/.config/remote-studio";
            GLib.mkdir_with_parents(dir, 0o755);
            GLib.file_set_contents(FAVORITES_FILE, JSON.stringify(this._favorites, null, 2));
        } catch(e) {
            global.logError("Remote Studio: Failed to save favorites: " + e);
        }
    },

    _toggleFavorite: function(profileKey) {
        let idx = this._favorites.indexOf(profileKey);
        if (idx >= 0) {
            this._favorites.splice(idx, 1);
        } else {
            this._favorites.push(profileKey);
        }
        this._saveFavorites();
        this._buildMenu();
    },

    // ── Session uptime tracking ──────────────────────────────────────────

    _loadSessionStart: function() {
        try {
            if (GLib.file_test(SESSION_START_FILE, GLib.FileTest.EXISTS)) {
                let [ok, buf] = GLib.file_get_contents(SESSION_START_FILE);
                if (ok) {
                    this._sessionStart = parseInt(buf.toString().trim(), 10);
                    if (!this._sessionStart || isNaN(this._sessionStart))
                        this._sessionStart = null;
                }
            }
        } catch(e) {
            this._sessionStart = null;
        }
    },

    _trackSessionStart: function() {
        this._sessionStart = Date.now();
        try {
            GLib.mkdir_with_parents(STATUS_DIR, 0o755);
            GLib.file_set_contents(SESSION_START_FILE, String(this._sessionStart));
        } catch(e) {}
    },

    _clearSessionUptime: function() {
        this._sessionStart = null;
        try {
            if (GLib.file_test(SESSION_START_FILE, GLib.FileTest.EXISTS))
                GLib.file_delete(SESSION_START_FILE);
        } catch(e) {}
    },

    _getSessionUptime: function() {
        if (this._sessionStart) {
            return Math.floor((Date.now() - this._sessionStart) / 1000);
        }
        return 0;
    },

    _formatUptime: function(seconds) {
        if (seconds <= 0) return "";
        let h = Math.floor(seconds / 3600);
        let m = Math.floor((seconds % 3600) / 60);
        let s = seconds % 60;
        if (h > 0) return h + "h " + m + "m";
        if (m > 0) return m + "m " + s + "s";
        return s + "s";
    },

    // ── Terminal detection ───────────────────────────────────────────────

    _findTerminal: function() {
        if (this._terminal) return this._terminal;
        for (let i = 0; i < TERMINAL_CANDIDATES.length; i++) {
            let found = GLib.find_program_in_path(TERMINAL_CANDIDATES[i]);
            if (found) {
                this._terminal = TERMINAL_CANDIDATES[i];
                return this._terminal;
            }
        }
        this._terminal = "xterm"; // ultimate fallback
        return this._terminal;
    },

    // ── Status file monitor (instant UI updates when daemon writes status) ───

    _setupStatusFileMonitor: function() {
        try {
            if (this._statusFileMonitor) return;
            let gfile = Gio.File.new_for_path(STATUS_FILE);
            let dir = Gio.File.new_for_path(STATUS_DIR);
            try {
                let monitor = dir.monitor(Gio.FileMonitorFlags.WATCH_MOVES, null);
                if (monitor) {
                    monitor.connect("changed", (mon, file, otherFile, eventType) => {
                        if (eventType === Gio.FileMonitorEvent.CHANGES_DONE_HINT ||
                            eventType === Gio.FileMonitorEvent.CHANGED ||
                            eventType === Gio.FileMonitorEvent.CREATED) {
                            this._readStatus();
                            this._scheduleUpdate();
                        }
                    });
                    this._statusFileMonitor = monitor;
                    global.log("Remote Studio: Status file monitor active");
                }
            } catch(e) {
                // Fall back to monitoring just the file
                let monitor = gfile.monitor(Gio.FileMonitorFlags.NONE, null);
                if (monitor) {
                    monitor.connect("changed", (mon, file, otherFile, eventType) => {
                        if (eventType === Gio.FileMonitorEvent.CHANGES_DONE_HINT ||
                            eventType === Gio.FileMonitorEvent.CHANGED) {
                            this._readStatus();
                            this._scheduleUpdate();
                        }
                    });
                    this._statusFileMonitor = monitor;
                    global.log("Remote Studio: Status file monitor active (file-level)");
                }
            }
        } catch(e) {
            global.logError("Remote Studio: Failed to set up status file monitor: " + e);
        }
    },

    // ── Status refresh ────────────────────────────────────────────────────────

    _scheduleUpdate: function() {
        if (this._timeout) { GLib.source_remove(this._timeout); this._timeout = null; }
        if (this._dbusProxy) {
            try {
                this._dbusProxy.RefreshRemote();
            } catch (e) {
                global.logError("Remote Studio: DBus RefreshRemote failed: " + e);
            }
        }

        // Read immediately, then poll as a fallback for missed events
        this._readStatus();
        this._timeout = GLib.timeout_add_seconds(GLib.PRIORITY_DEFAULT, 5, () => {
            this._readStatus();
            return GLib.SOURCE_CONTINUE;
        });

        // Ensure status file monitor is active
        this._setupStatusFileMonitor();
    },
    _setupDBus: function() {
        try {
            // Watch for name owner changes for auto-reconnection
            if (this._dbusWatchId === undefined) {
                this._dbusWatchId = Gio.DBus.session.watch_name(
                    'org.remote_studio.Daemon',
                    Gio.BusNameWatcherFlags.NONE,
                    (connection, name, nameOwner) => {
                        if (nameOwner) {
                            global.log("Remote Studio: DBus daemon appeared (" + nameOwner + ")");
                            // Daemon appeared (or re-appeared) — set up proxy
                            this._connectDBusProxy();
                        } else {
                            global.log("Remote Studio: DBus daemon vanished, will retry");
                            this._dbusProxy = null;
                            if (this._dbusReconnect !== false) {
                                // Schedule a reconnection attempt in 3 seconds
                                if (this._dbusRetryTimeout) {
                                    GLib.source_remove(this._dbusRetryTimeout);
                                }
                                this._dbusRetryTimeout = GLib.timeout_add(
                                    GLib.PRIORITY_DEFAULT, 3000, () => {
                                        this._dbusRetryTimeout = null;
                                        global.log("Remote Studio: Retrying DBus connection...");
                                        this._setupDBus();
                                        return GLib.SOURCE_REMOVE;
                                    }
                                );
                            }
                        }
                    }
                );
            }

            if (!this._dbusProxy) {
                this._connectDBusProxy();
            }
        } catch (e) {
            global.logError("Remote Studio: DBus setup failed: " + e);
        }
    },

    _connectDBusProxy: function() {
        try {
            if (this._dbusProxySignalId && this._dbusProxy) {
                try {
                    this._dbusProxy.disconnectSignal(this._dbusProxySignalId);
                } catch(e) {}
                this._dbusProxySignalId = null;
            }

            this._dbusProxy = new DBusProxy(
                Gio.DBus.session,
                'org.remote_studio.Daemon',
                '/org/remote_studio/Daemon',
                (proxy, error) => {
                    if (error) {
                        global.logError("Failed to connect to Daemon DBus: " + error.message);
                    } else {
                        try {
                            this._readStatusFromDBus(proxy.Status);
                        } catch(e) {}
                    }
                }
            );

            this._dbusProxySignalId = this._dbusProxy.connectSignal('StatusChanged', (proxy, senderName, [statusString]) => {
                this._readStatusFromDBus(statusString);
            });

            global.log("Remote Studio: DBus proxy connected successfully");
        } catch (e) {
            global.logError("Remote Studio: DBus proxy connect failed: " + e);
        }
    },

    // ── State readers ─────────────────────────────────────────────────────────

    _getCurrentMode: function() {
        try {
            let [ok, buf] = GLib.file_get_contents(STATE_FILE);
            if (ok) { let m = buf.toString().trim().match(/'([^']+)'/); return m ? m[1] : ""; }
        } catch(e) {}
        return "";
    },

    _getCurrentResolution: function() {
        try {
            let [ok, buf] = GLib.file_get_contents(STATE_FILE);
            if (ok) {
                let fields = buf.toString().trim().split(/\s+/);
                if (fields.length >= 2 && fields[0].match(/^\d+$/) && fields[1].match(/^\d+$/))
                    return fields[0] + "×" + fields[1];
            }
        } catch(e) {}
        return "";
    },

    _getDirectAddress: function() {
        if (this._lastStatus) return this._lastStatus.direct;
        try {
            let [ok, buf] = GLib.file_get_contents(STATUS_FILE);
            if (ok) {
                let status = this._parseStatus(buf.toString());
                return status ? status.direct : null;
            }
        } catch(e) {}
        return null;
    },

    _getActiveIPs: function() {
        if (this._lastStatus) return this._lastStatus.active_ips || [];
        try {
            let [ok, buf] = GLib.file_get_contents(STATUS_FILE);
            if (ok) {
                let status = this._parseStatus(buf.toString());
                return status ? (status.active_ips || []) : [];
            }
        } catch(e) {}
        return [];
    },

    // ── Panel label / icon ────────────────────────────────────────────────────

    _iconForMode: function(modeName) {
        for (let k in MODE_ICONS) { if (modeName.indexOf(k) !== -1) return MODE_ICONS[k]; }
        if (modeName && modeName.indexOf("Custom") === 0) return "preferences-desktop-display-symbolic";
        return "video-display-symbolic";
    },

    _applyLabelColor: function(color) {
        try {
            this.actor.style = 'color: ' + color + ';';
        } catch(e) {}
    },

    _panelColorForStatus: function(status) {
        if (!status) return "#ff6600";          // orange — unknown / error
        if (status.warnings > 0) return "#ffaa00";   // yellow — warnings present
        if (status.users > 0) {
            return status.connType === "Direct"
                ? "#00cc66"  // green — direct connection
                : "#66bbff"; // blue — relayed connection
        }
        // Session active but no users
        if (status.label && status.label !== "Unknown" && status.label !== "None"
            && status.label.indexOf("Reset") === -1) {
            return "#aaaaaa"; // gray — active but idle
        }
        return "#888888"; // dim — no session
    },

    _ellipsize: function(text, maxLength) {
        text = (text || "").trim();
        if (text.length <= maxLength) return text;
        if (maxLength <= 1) return text.slice(0, maxLength);
        return text.slice(0, maxLength - 1).trim() + "…";
    },

    _compactModeLabel: function(label) {
        label = (label || "Unknown").replace(/\s*\([^)]*\)\s*/g, " ").replace(/\s+/g, " ").trim();
        return this._ellipsize(label, MAX_PANEL_LABEL);
    },

    _panelLabel: function(status) {
        let dot = status.users > 0 ? (status.connType === "Direct" ? "● " : "◐ ") : "";
        let alert = status.warnings > 0 ? "⚠ " : "";
        let uc = status.users > 0 ? " 👥" + status.users : "";
        let uptime = this._getSessionUptime();
        let uptimeStr = (uptime > 60 && this._uptimeDisplay !== false) ? " ⏱" + this._formatUptime(uptime) : "";
        let base = alert + dot + this._compactModeLabel(status.label) + uc + uptimeStr;

        if (base.length > MAX_PANEL_LABEL) {
            let reserved = alert.length + dot.length + uc.length + uptimeStr.length;
            let maxLabel = Math.max(0, MAX_PANEL_LABEL - reserved);
            base = alert + dot + this._ellipsize(this._compactModeLabel(status.label), maxLabel) + uc + uptimeStr;
        }
        return base;
    },

    _parseStatus: function(raw) {
        raw = (raw || "").trim();
        if (!raw) return null;

        if (raw[0] === "{") {
            try {
                let j = JSON.parse(raw);
                return {
                    label: (j.mode || "Unknown").toString(),
                    temp: (j.temperature || "N/A").toString(),
                    latency: (j.latency || "N/A").toString(),
                    users: parseInt(j.users, 10) || 0,
                    ram: (j.ram || "N/A").toString(),
                    warnings: parseInt((j.warnings && j.warnings.count) || 0, 10) || 0,
                    warningText: ((j.warnings && j.warnings.summary) || "none").toString(),
                    traffic: (j.network || "N/A").toString(),
                    ip: (j.ip || "N/A").toString(),
                    connType: (j.connection || "N/A").toString(),
                    resolution: (j.resolution || "N/A").toString(),
                    direct: (j.direct_address || "").toString().trim() || null,
                    codec: (j.codec || "").toString().trim(),
                    active_ips: j.active_ips || []
                };
            } catch(e) {
                return null;
            }
        }

        let p = raw.split(" | ");
        if (p.length < 9) return null;
        return {
            label: (p[0] || "Unknown").trim(),
            temp: (p[1] || "N/A").trim(),
            latency: (p[2] || "N/A").trim(),
            users: parseInt(p[3], 10) || 0,
            ram: (p[4] || "N/A").trim(),
            warnings: parseInt(p[5], 10) || 0,
            warningText: (p[6] || "none").trim(),
            traffic: (p[7] || "N/A").trim(),
            ip: (p[8] || "N/A").trim(),
            connType: (p[9] || "N/A").trim(),
            resolution: (p[10] || "N/A").trim(),
            direct: (p[11] || "").trim() || null,
            codec: ((p[12] || "").trim() === "none") ? "" : (p[12] || "").trim(),
            active_ips: []
        };
    },

    _setUnavailable: function(reason) {
        this.set_applet_icon_name("dialog-warning-symbolic");
        this.set_applet_label("Res ?");
        this.set_applet_tooltip("Remote Studio\n" + reason + "\nStatus: " + STATUS_FILE);
        this._applyLabelColor("#ff4444");
    },

    _readStatus: function() {
        try {
            let [ok, buf] = GLib.file_get_contents(STATUS_FILE);
            if (!ok) return;
            this._applyStatus(buf.toString());
        } catch(e) {
            this._setUnavailable("Status read failed");
        }
    },

    _applyStatus: function(raw) {
        try {
            let status = this._parseStatus(raw);
            if (!status) {
                this._setUnavailable("Status data is incomplete");
                return;
            }

            this._lastStatus = status;
            this._activeIPs = status.active_ips || [];

            let users = status.users;
            let prevUsers = this.lastUserCount;

            // ── Auto-session: start session when connection detected ──────
            if (this.autoSession && users > 0 && prevUsers === 0) {
                let profile = this._autoSessionProfile || "last";
                if (profile === "last" && this._lastUsedProfile) {
                    profile = this._lastUsedProfile;
                } else if (profile === "last") {
                    profile = this._getDefaultProfile();
                }
                global.log("Remote Studio: Auto-starting session with '" + profile + "' due to connection");
                this._runRes("session start " + profile);
                // Track session start for uptime (runRes also does this but double-tracking is safe)
            }

            if (this._notifyEnabled) {
                if (users > prevUsers)
                    Util.spawn(["notify-send", "-u", "critical", "Remote Studio",
                        (users - prevUsers) + " user(s) connected"]);
                else if (users < prevUsers && prevUsers > 0)
                    Util.spawn(["notify-send", "-u", "normal", "Remote Studio",
                        "User disconnected (" + users + " remaining)"]);
            }
            this.lastUserCount = users;

            this.set_applet_icon_name(this._iconForMode(status.label));
            this.set_applet_label(this._panelLabel(status));

            // ── Latency quality indicator ─────────────────────────────────
            let latencyBars = "";
            if (this._showLatencyBars !== false && status.latency && status.latency !== "N/A" && status.latency !== "-") {
                let latVal = parseInt(status.latency, 10);
                if (!isNaN(latVal) && latVal > 0) {
                    if (latVal <= 30)          { latencyBars = " ▂▄▆█"; }  // excellent
                    else if (latVal <= 60)     { latencyBars = " ▂▄▆◌"; }  // good
                    else if (latVal <= 100)    { latencyBars = " ▂▄◌◌"; }  // fair
                    else                        { latencyBars = " ▂◌◌◌"; }  // poor
                }
            }

            this.set_applet_tooltip(
                "Remote Studio"   +
                "\nMode: "    + status.label      +
                "\nRes: "     + status.resolution +
                "\nPath: "    + status.connType   +
                (status.codec ? "\nCodec: " + status.codec : "") +
                "\nIP: "      + status.ip         +
                "\nDirect: "  + (status.direct || "N/A") +
                "\nTemp: "    + status.temp       +
                "\nRAM: "     + status.ram        +
                "\nLatency: " + status.latency    +
                latencyBars +
                "\nTraffic: " + status.traffic    +
                "\nWarnings: "+ status.warningText +
                (this.lastUserCount > 0 ? "\nActive: " + this._activeIPs.join(", ") : "")
            );

            // Color-coded label (respects settings toggle)
            if (this._labelColorEnabled !== false) {
                this._applyLabelColor(this._panelColorForStatus(status));
            } else {
                this._applyLabelColor("");
            }
        } catch(e) {
            this._setUnavailable("Status update failed");
        }
    },

    // DBus status handler — the daemon owns the status file, so apply the
    // pushed payload directly instead of writing it back to disk
    _readStatusFromDBus: function(statusString) {
        try {
            if (!statusString) return;
            this._applyStatus(statusString);
        } catch(e) {
            global.logError("Remote Studio: _readStatusFromDBus failed: " + e);
        }
    },

    // ── Profile loading (deduped, last-wins, insertion order, with caching) ──

    _loadProfiles: function(force) {
        // Cache for 5 seconds unless forced
        if (this._profileCache && !force &&
            (Date.now() - this._profileCacheTime) < 5000) {
            return this._profileCache;
        }

        let seen = {}, order = [];
        [PROFILES_FILE, USER_PROFILES_FILE].forEach(file => {
            try {
                if (!GLib.file_test(file, GLib.FileTest.EXISTS)) return;
                let [ok, buf] = GLib.file_get_contents(file);
                if (!ok) return;
                buf.toString().split("\n").forEach(line => {
                    if (!line || line.indexOf("#") === 0 || line.indexOf("=") === -1) return;
                    let eq   = line.indexOf("=");
                    let key  = line.slice(0, eq).trim();
                    let val  = line.slice(eq + 1);
                    let [label, w, h, scale] = val.split("|");
                    seen[key] = { key, label: (label||"").trim(), width: w, height: h, scale };
                    if (order.indexOf(key) === -1) order.push(key);
                });
            } catch(e) {}
        });

        this._profileCache = order.map(k => seen[k]);
        this._profileCacheTime = Date.now();

        // Set up file monitors for auto-refresh (once)
        this._setupProfileMonitors();

        return this._profileCache;
    },

    _setupProfileMonitors: function() {
        // Only set up once
        if (this._profileMonitorsSetup) return;
        this._profileMonitorsSetup = true;

        [PROFILES_FILE, USER_PROFILES_FILE].forEach(file => {
            try {
                if (!GLib.file_test(file, GLib.FileTest.EXISTS)) return;
                let gfile = Gio.File.new_for_path(file);
                let monitor = gfile.monitor(Gio.FileMonitorFlags.NONE, null);
                if (monitor) {
                    monitor.connect("changed", (mon, f, otherFile, eventType) => {
                        if (eventType === Gio.FileMonitorEvent.CHANGES_DONE_HINT ||
                            eventType === Gio.FileMonitorEvent.CHANGED) {
                            this._profileCache = null; // invalidate cache
                            if (this.menu && this.menu.isOpen) {
                                this._buildMenu();
                            }
                        }
                    });
                    this._profilesMonitors.push(monitor);
                }
            } catch(e) {}
        });
    },

    // ── Action runner ─────────────────────────────────────────────────────────

    _runRes: function(arg) {
        if (!GLib.file_test(RES_CMD, GLib.FileTest.EXISTS)) {
            this._setUnavailable("res command not found at " + RES_CMD);
            Util.spawn(["notify-send", "-u", "critical", "Remote Studio",
                "res command not found at " + RES_CMD + " — is Remote Studio installed?"]);
            return;
        }
        Util.spawn([RES_CMD].concat(arg.split(" ")));

        // Track session start/stop and last used profile
        let args = arg.split(" ");
        if (args.length >= 3 && args[0] === "session" && args[1] === "start") {
            this._trackSessionStart();
            // Store last used profile (the third arg is the profile key)
            this._lastUsedProfile = args[2];
            try {
                let lastFile = GLib.get_home_dir() + "/.config/remote-studio/.last_used_profile";
                GLib.mkdir_with_parents(GLib.get_home_dir() + "/.config/remote-studio", 0o755);
                GLib.file_set_contents(lastFile, this._lastUsedProfile);
            } catch(e) {}
        } else if (arg === "session stop") {
            this._clearSessionUptime();
        }

        // Refresh label and menu after action settles
        GLib.timeout_add(GLib.PRIORITY_DEFAULT, 1200, () => {
            this._scheduleUpdate();
            this._buildMenu();
            return GLib.SOURCE_REMOVE;
        });
    },

    _getLastUsedProfile: function() {
        if (this._lastUsedProfile) return this._lastUsedProfile;
        try {
            let f = GLib.get_home_dir() + "/.config/remote-studio/.last_used_profile";
            if (GLib.file_test(f, GLib.FileTest.EXISTS)) {
                let [ok, buf] = GLib.file_get_contents(f);
                if (ok) {
                    let val = buf.toString().trim();
                    if (val) { this._lastUsedProfile = val; return val; }
                }
            }
        } catch(e) {}
        return null;
    },

    // ── Clipboard ────────────────────────────────────────────────────────

    _copyToClipboard: function(text) {
        try {
            St.Clipboard.get_default().set_text(St.ClipboardType.CLIPBOARD, text);
        } catch(e) {
            // Older Cinnamon: set_text without a type argument
            try {
                St.Clipboard.get_default().set_text(text);
            } catch(e2) {
                Util.spawn(["bash", "-c",
                    "printf %s \"$1\" | xclip -selection clipboard",
                    "remote-studio", text]);
            }
        }
    },

    // ── Confirmation pattern ─────────────────────────────────────────────

    _requestConfirm: function(key, label, callback) {
        if (this._confirmItems[key]) {
            // Already in confirmation mode — execute on second click
            GLib.source_remove(this._confirmItems[key].timeoutId);
            delete this._confirmItems[key];
            callback();
            this._buildMenu();
            return;
        }

        // Show "Confirm?" state — auto-cancels after CONFIRM_TIMEOUT_MS
        this._confirmItems[key] = {
            timeoutId: GLib.timeout_add(GLib.PRIORITY_DEFAULT, CONFIRM_TIMEOUT_MS, () => {
                delete this._confirmItems[key];
                if (this.menu && this.menu.isOpen) this._buildMenu();
                return GLib.SOURCE_REMOVE;
            }),
            callback: callback
        };
        this._buildMenu();
        // Activating an item closes the menu; reopen it so the
        // "tap again to confirm" state is actually visible
        if (this.menu && !this.menu.isOpen) this.menu.open(false);
    },

    _isConfirming: function(key) {
        return !!this._confirmItems[key];
    },

    // ── Submenu helpers ───────────────────────────────────────────────────────

    // Create a collapsible group; persists open/closed state across rebuilds
    _makeGroup: function(title, stateKey) {
        let group = new PopupMenu.PopupSubMenuMenuItem(title);
        if (this._groupOpen[stateKey]) group.menu.open(false);
        group.menu.connect("open-state-changed", (menu, open) => {
            this._groupOpen[stateKey] = open;
        });
        return group;
    },

    _subItem: function(group, label, icon, arg) {
        let item = new PopupMenu.PopupIconMenuItem(label, icon, St.IconType.SYMBOLIC);
        item.connect("activate", () => this._runRes(arg));
        group.menu.addMenuItem(item);
        return item;
    },

    _subTerminal: function(group, label, icon, arg) {
        let item = new PopupMenu.PopupIconMenuItem(label, icon, St.IconType.SYMBOLIC);
        item.connect("activate", () => {
            Util.spawn([this._findTerminal(), "-e", RES_CMD].concat(arg.split(" ")));
        });
        group.menu.addMenuItem(item);
    },

    _subSep: function(group) {
        group.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());
    },

    _subConfirmItem: function(group, label, icon, confirmKey, actionArg, destructiveLabel) {
        // Respect the "confirm before destructive actions" setting
        if (this._confirmDestructive === false) {
            return this._subItem(group, label, icon, actionArg);
        }
        let isConfirming = this._isConfirming(confirmKey);
        let itemLabel = isConfirming ? (destructiveLabel || "⚠ Confirm?") : label;
        let itemIcon  = isConfirming ? "dialog-warning-symbolic" : icon;
        let item = new PopupMenu.PopupIconMenuItem(itemLabel, itemIcon, St.IconType.SYMBOLIC);
        if (isConfirming) {
            item.label.style = 'color: #ff4444; font-weight: bold;';
        }
        item.connect("activate", () => {
            this._requestConfirm(confirmKey, label, () => {
                this._runRes(actionArg);
            });
        });
        group.menu.addMenuItem(item);
        return item;
    },

    // ── Menu builder ──────────────────────────────────────────────────────────

    _buildMenu: function() {
        this.menu.removeAll();

        let currentMode    = this._getCurrentMode();
        let profiles       = this._loadProfiles(false);
        let defaultProfile = this._getDefaultProfile();
        let directAddr     = this._getDirectAddress();
        let activeIPs      = this._getActiveIPs();
        let uptime         = this._getSessionUptime();

        // Smart: if the current mode belongs to a profile, ensure presets group
        // is open so the active checkmark is visible
        let activeIsInProfiles = profiles.some(p => p.label === currentMode);
        if (activeIsInProfiles) this._groupOpen.presets = true;

        // ── 👥 Active Sessions ────────────────────────────────────────────
        if (activeIPs && activeIPs.length > 0) {
            let activeGroup = this._makeGroup("👥  Active Sessions (" + activeIPs.length + ")", "active_sessions");
            activeIPs.forEach(ip => {
                let ipItem = new PopupMenu.PopupIconMenuItem(
                    "🔌  " + ip, "network-workgroup-symbolic", St.IconType.SYMBOLIC
                );
                ipItem.actor.set_reactive(false);
                activeGroup.menu.addMenuItem(ipItem);
            });
            this._subSep(activeGroup);
            this._subConfirmItem(activeGroup, "⏹  Disconnect All",
                "media-playback-stop-symbolic", "disconnect_all", "session stop", "⛔  Tap again to disconnect");
            this.menu.addMenuItem(activeGroup);
        }

        // ── 📺 Device Presets ─────────────────────────────────────────────────
        let presetsGroup = this._makeGroup("📺  Device Presets", "presets");

        // Sort: favorites first, then alphabetically
        let sortedProfiles = [...profiles].sort((a, b) => {
            let aFav = this._favorites.indexOf(a.key) >= 0 ? 0 : 1;
            let bFav = this._favorites.indexOf(b.key) >= 0 ? 0 : 1;
            if (aFav !== bFav) return aFav - bFav;
            return (a.label || "").localeCompare(b.label || "");
        });

        sortedProfiles.forEach(p => {
            let active = (currentMode === p.label);
            let mark   = active ? "✓  " : "     ";
            let star   = this._favorites.indexOf(p.key) >= 0 ? "★ " : "";
            let icon   = this._iconForMode(p.label);
            let item   = new PopupMenu.PopupIconMenuItem(
                mark + star + p.label + "  " + p.width + "×" + p.height,
                icon, St.IconType.SYMBOLIC
            );
            if (active) item.setShowDot(true);
            item.connect("activate", () => this._runRes(p.key));

            // Right-click to toggle favorite
            item.actor.connect("button-release-event", (actor, event) => {
                if (event.get_button() === 3) { // Right mouse button
                    this._toggleFavorite(p.key);
                    return true;
                }
                return false;
            });

            presetsGroup.menu.addMenuItem(item);
        });
        this.menu.addMenuItem(presetsGroup);

        // ── Custom Resolution ─────────────────────────────────────────────
        let customItem = new PopupMenu.PopupIconMenuItem(
            "✏  Custom Resolution…", "preferences-desktop-display-symbolic", St.IconType.SYMBOLIC
        );
        customItem.connect("activate", () => this._promptCustomResolution());
        presetsGroup.menu.addMenuItem(customItem);

        // ── ⚡ Performance & Session ──────────────────────────────────────────
        let perfGroup = this._makeGroup("⚡  Performance & Session", "performance");

        // Show uptime if session is active
        if (uptime > 0) {
            let uptimeStr = this._formatUptime(uptime);
            let uptimeItem = new PopupMenu.PopupIconMenuItem(
                "⏱  Session Uptime: " + uptimeStr,
                "alarm-symbolic", St.IconType.SYMBOLIC
            );
            uptimeItem.actor.set_reactive(false);
            perfGroup.menu.addMenuItem(uptimeItem);
            this._subSep(perfGroup);
        }

        this._subItem(perfGroup, "▶  Start Session  (" + defaultProfile + ")",
            "media-playback-start-symbolic", "session start " + defaultProfile);
        this._subConfirmItem(perfGroup, "⏹  Stop Session",
            "media-playback-stop-symbolic", "stop_session", "session stop", "⛔  Tap again to stop");
        this._subSep(perfGroup);
        this._subItem(perfGroup, "Toggle Speed Mode",  "go-jump-symbolic",              "speed");
        this._subItem(perfGroup, "Toggle Caffeine",    "battery-symbolic",              "caf");
        this._subItem(perfGroup, "Toggle Theme",       "weather-clear-night-symbolic",  "theme");
        this._subItem(perfGroup, "Toggle Night Shift", "night-light-symbolic",          "night");
        this.menu.addMenuItem(perfGroup);

        // ── 📡 RustDesk ───────────────────────────────────────────────────────
        let rdGroup = this._makeGroup("📡  RustDesk", "rustdesk");
        this._subItem(rdGroup, "Quality Preset",  "video-display-symbolic", "rustdesk apply quality");
        this._subItem(rdGroup, "Balanced Preset", "video-display-symbolic", "rustdesk apply balanced");
        this._subItem(rdGroup, "Speed Preset",    "video-display-symbolic", "rustdesk apply speed");
        this._subSep(rdGroup);
        this._subItem(rdGroup, "Restart Service", "network-server-symbolic", "service");
        this.menu.addMenuItem(rdGroup);

        // ── 🛠 System & Security ──────────────────────────────────────────────
        let sysGroup = this._makeGroup("🛠  System & Security", "system");
        this._subItem(sysGroup,    "Privacy Shield (Lock)",        "system-lock-screen-symbolic",  "privacy");
        this._subItem(sysGroup,    "Fix Clipboard / Audio / Keys", "applications-system-symbolic",  "fix");
        this._subTerminal(sysGroup,"Run Doctor",                   "dialog-information-symbolic",   "doctor");
        this._subTerminal(sysGroup,"Show Tailnet Address",         "network-vpn-symbolic",          "tailnet");
        this._subTerminal(sysGroup,"Tailnet Doctor",               "network-wired-symbolic",        "tailnet doctor");
        this._subConfirmItem(sysGroup, "Emergency Display Reset", "view-refresh-symbolic",
            "display_reset", "reset", "⛔  Tap again to reset");
        this.menu.addMenuItem(sysGroup);

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        // ── Footer (always visible) ───────────────────────────────────────────

        if (directAddr) {
            let copyItem = new PopupMenu.PopupIconMenuItem(
                "⎘  Copy Direct Address (" + directAddr + ")",
                "edit-copy-symbolic", St.IconType.SYMBOLIC
            );
            copyItem.connect("activate", () => this._copyToClipboard(directAddr));
            this.menu.addMenuItem(copyItem);
        }

        let muteLabel = this._notifyEnabled ? "🔕  Mute Connect/Disconnect Alerts" : "🔔  Unmute Alerts";
        let muteIcon  = this._notifyEnabled ? "notifications-disabled-symbolic" : "notification-symbolic";
        let muteItem  = new PopupMenu.PopupIconMenuItem(muteLabel, muteIcon, St.IconType.SYMBOLIC);
        muteItem.connect("activate", () => {
            this._notifyEnabled = !this._notifyEnabled;
            if (this.settings) {
                try { this.settings.setValue("notify_enabled", this._notifyEnabled); } catch(e) {}
            }
            this._buildMenu();
        });
        this.menu.addMenuItem(muteItem);

        let tuiItem = new PopupMenu.PopupIconMenuItem(
            "⌨  Open Full TUI Dashboard", "utilities-terminal-symbolic", St.IconType.SYMBOLIC
        );
        tuiItem.connect("activate", () => { Util.spawn([this._findTerminal(), "-e", RES_CMD]); });
        this.menu.addMenuItem(tuiItem);
    },

    // ── Custom Resolution Prompt ──────────────────────────────────────────

    _promptCustomResolution: function() {
        if (!GLib.find_program_in_path("zenity")) {
            // No dialog tool available — fall back to the TUI in a terminal
            Util.spawn([this._findTerminal(), "-e", RES_CMD]);
            return;
        }
        let currentRes = this._getCurrentResolution();
        if (!currentRes) currentRes = "1920x1080";
        Util.spawn(['bash', '-c',
            'RESULT=$(zenity --entry --title="Remote Studio: Custom Resolution" ' +
            '--text="Enter WxH (e.g. 1920x1080 or 1920×1080):" ' +
            '--entry-text="' + currentRes + '" 2>/dev/null) && ' +
            'if [ -n "$RESULT" ]; then ' +
            '  RESULT=$(echo "$RESULT" | tr "×" "x" | tr -d " "); ' +
            '  echo "$RESULT" | grep -qE "^[0-9]+x[0-9]+$" && ' +
            '  ' + RES_CMD + ' custom "$RESULT"; ' +
            'fi'
        ]);
        // Refresh after a short delay to pick up new state
        GLib.timeout_add(GLib.PRIORITY_DEFAULT, 1500, () => {
            this._scheduleUpdate();
            this._buildMenu();
            return GLib.SOURCE_REMOVE;
        });
    },

    // ── Applet lifecycle ──────────────────────────────────────────────────────

    _handleMiddleClick: function() {
        let action = this._middleClickAction || "menu";
        switch (action) {
            case "speed":
                this._runRes("speed");
                break;
            case "night":
                this._runRes("night");
                break;
            case "session":
                if (this._sessionStart) {
                    this._clearSessionUptime();
                    this._runRes("session stop");
                } else {
                    let defaultProfile = this._getDefaultProfile();
                    this._trackSessionStart();
                    this._runRes("session start " + defaultProfile);
                }
                break;
            case "privacy":
                this._runRes("privacy");
                break;
            case "copy": {
                let directAddr = this._getDirectAddress();
                if (directAddr) this._copyToClipboard(directAddr);
                break;
            }
            case "menu":
            default:
                this._buildMenu();
                this._scheduleUpdate();
                this.menu.toggle();
                break;
        }

        if (action !== "menu") {
            // Show brief visual feedback by triggering quick update
            GLib.timeout_add(GLib.PRIORITY_DEFAULT, 300, () => {
                this._readStatus();
                return GLib.SOURCE_REMOVE;
            });
        }
    },

    on_applet_clicked: function(event) {
        // Debounce: ignore rapid clicks within 200ms
        let now = Date.now();
        if (now - this._lastClick < 200) return;
        this._lastClick = now;

        // Check for middle-click (button 2) to trigger quick action
        if (event && event.get_button) {
            let button = event.get_button();
            if (button === 2) {
                this._handleMiddleClick();
                return;
            }
            // Right-click behavior
            if (button === 3) {
                // Same as left click for now — opens menu
            }
        }

        this._buildMenu();
        this._scheduleUpdate();
        this.menu.toggle();
    },

    on_applet_middle_clicked: function() {
        this._handleMiddleClick();
    },

    on_applet_removed_from_panel: function() {
        if (this._timeout) {
            GLib.source_remove(this._timeout);
            this._timeout = null;
        }
        if (this._dbusWatchId !== undefined) {
            try { Gio.DBus.session.unwatch_name(this._dbusWatchId); } catch(e) {}
        }
        if (this._dbusRetryTimeout) {
            GLib.source_remove(this._dbusRetryTimeout);
            this._dbusRetryTimeout = null;
        }
        if (this._dbusProxy && this._dbusProxySignalId) {
            this._dbusProxy.disconnectSignal(this._dbusProxySignalId);
        }
        if (this._statusFileMonitor) {
            try { this._statusFileMonitor.cancel(); } catch(e) {}
            this._statusFileMonitor = null;
        }
        this._profilesMonitors.forEach(m => { try { m.cancel(); } catch(e) {} });
        this._profilesMonitors = [];
        // Cancel any pending confirmations
        for (let key in this._confirmItems) {
            GLib.source_remove(this._confirmItems[key].timeoutId);
        }
        this._confirmItems = {};
        // Clean up settings
        if (this.settings) {
            try { this.settings.finalize(); } catch(e) {}
        }
    },
};

function main(metadata, orientation, panel_height, instance_id) {
    return new MyApplet(metadata, orientation, panel_height, instance_id);
}