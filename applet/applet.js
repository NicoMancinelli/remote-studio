const Applet    = imports.ui.applet;
const PopupMenu = imports.ui.popupMenu;
const Settings  = imports.ui.settings;
const Util      = imports.misc.util;
const GLib      = imports.gi.GLib;
const Gio       = imports.gi.Gio;
const St        = imports.gi.St;

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

const _groupOpen = { presets: true, performance: false, rustdesk: false, system: false };

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

        this.menuManager    = new PopupMenu.PopupMenuManager(this);
        this.menu           = new Applet.AppletPopupMenu(this, orientation);
        this.menuManager.addMenu(this.menu);

        this.lastUserCount  = 0;
        this._lastStatus    = null;
        this._timeout       = null;
        this._fileMonitor   = null;
        this._refreshInFlight = false;

        this.settings = new Settings.AppletSettings(this, metadata.uuid, instance_id);
        this.settings.bind("default_profile", "defaultProfile", this._onSettingsChanged);
        this.settings.bind("auto_session", "autoSession", this._onSettingsChanged);
        this.settings.bind("notify_enabled", "_notifyEnabled", this._buildMenu);

        this._scheduleUpdate();
        this._setupFileWatch();
        this._buildMenu();
    },

    _onSettingsChanged: function() {
        Util.spawn([RES_CMD, "config", "set", "DEFAULT_PROFILE", this.defaultProfile || "mac"]);
        Util.spawn([RES_CMD, "config", "set", "AUTO_SESSION", this.autoSession ? "true" : "false"]);
        if (this.menu && this.menu.isOpen) this._buildMenu();
    },

    _scheduleUpdate: function() {
        if (this._timeout) GLib.source_remove(this._timeout);

        if (!this._refreshInFlight) {
            this._refreshInFlight = true;
            try {
                let [ok, pid] = GLib.spawn_async(null,
                    ["/bin/bash", "-c",
                     "mkdir -p " + GLib.shell_quote(STATUS_DIR) +
                     " && " + GLib.shell_quote(RES_CMD) +
                     " status > " + GLib.shell_quote(STATUS_FILE)],
                    null,
                    GLib.SpawnFlags.SEARCH_PATH | GLib.SpawnFlags.DO_NOT_REAP_CHILD,
                    null, null);
                if (ok) {
                    GLib.child_watch_add(GLib.PRIORITY_DEFAULT, pid, () => {
                        this._refreshInFlight = false;
                        GLib.spawn_close_pid(pid);
                    });
                } else {
                    this._refreshInFlight = false;
                }
            } catch(e) {
                this._refreshInFlight = false;
                this._setUnavailable("Status refresh failed");
            }
        }

        this._timeout = GLib.timeout_add_seconds(GLib.PRIORITY_DEFAULT, 10, () => {
            this._timeout = null;
            this._scheduleUpdate();
            return GLib.SOURCE_REMOVE;
        });
    },

    _setupFileWatch: function() {
        try {
            let sf  = Gio.File.new_for_path(STATUS_FILE);
            let sd  = Gio.File.new_for_path(STATUS_DIR);
            if (!sd.query_exists(null)) sd.make_directory_with_parents(null);
            if (!sf.query_exists(null))
                sf.replace_contents("", null, false, Gio.FileCreateFlags.NONE, null);
            this._fileMonitor = sf.monitor_file(Gio.FileMonitorFlags.NONE, null);
            this._fileMonitor.connect("changed", (monitor, file, other, ev) => {
                if (ev === Gio.FileMonitorEvent.CHANGES_DONE_HINT ||
                    ev === Gio.FileMonitorEvent.MODIFIED)
                    this._readStatus();
            });
        } catch(e) { /* polling fallback via _scheduleUpdate */ }
    },

    _getCurrentMode: function() {
        try {
            let [ok, buf] = GLib.file_get_contents(STATE_FILE);
            if (ok) { let m = buf.toString().trim().match(/'([^']+)'/); return m ? m[1] : ""; }
        } catch(e) {}
        return "";
    },

    _getDefaultProfile: function() {
        if (this.defaultProfile) return this.defaultProfile;
        try {
            let f = GLib.get_home_dir() + "/.config/remote-studio/remote-studio.conf";
            if (GLib.file_test(f, GLib.FileTest.EXISTS)) {
                let [ok, buf] = GLib.file_get_contents(f);
                if (ok) { let m = buf.toString().match(/^DEFAULT_PROFILE=(.+)$/m); if (m) return m[1].trim(); }
            }
        } catch(e) {}
        return "mac";
    },

    _iconForMode: function(modeName) {
        for (let k in MODE_ICONS) { if (modeName.indexOf(k) !== -1) return MODE_ICONS[k]; }
        if (modeName && modeName.indexOf("Custom") === 0) return "preferences-desktop-display-symbolic";
        return "video-display-symbolic";
    },

    _ellipsize: function(text, maxLength) {
        text = (text || "").trim();
        if (text.length <= maxLength) return text;
        if (maxLength <= 1) return text.slice(0, maxLength);
        return text.slice(0, maxLength - 1).trim() + "\u2026";
    },

    _compactModeLabel: function(label) {
        label = (label || "Unknown").replace(/\s*\([^)]*\)\s*/g, " ").replace(/\s+/g, " ").trim();
        return this._ellipsize(label, MAX_PANEL_LABEL);
    },

    _shortCodec: function(codec) {
        if (!codec) return "";
        let c = codec.toUpperCase();
        if (c.indexOf("H265") !== -1 || c.indexOf("HEVC") !== -1) return "H265";
        if (c.indexOf("H264") !== -1 || c.indexOf("AVC") !== -1) return "H264";
        if (c.indexOf("VP9") !== -1) return "VP9";
        if (c.indexOf("VP8") !== -1) return "VP8";
        if (c.indexOf("AV1") !== -1) return "AV1";
        return codec.length > 6 ? codec.slice(0, 6) : codec;
    },

    _panelLabel: function(status) {
        let dot = status.users > 0 ? (status.connType === "Direct" ? "\u25cf " : "\u25d0 ") : "";
        let alert = status.warnings > 0 ? "! " : "";
        let users = status.users > 0 ? " \u00d7" + status.users : "";
        let codec = (status.users > 0 && status.codec) ? " " + this._shortCodec(status.codec) : "";
        let mode = this._compactModeLabel(status.label);
        let base = alert + dot + mode + codec + users;

        if (base.length > MAX_PANEL_LABEL) {
            let reserved = alert.length + dot.length + users.length + codec.length;
            base = alert + dot + this._ellipsize(mode, Math.max(4, MAX_PANEL_LABEL - reserved)) + codec + users;
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
                    codec: (j.codec || "").toString().trim()
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
            codec: ((p[12] || "").trim() === "none") ? "" : (p[12] || "").trim()
        };
    },

    _setUnavailable: function(reason) {
        this.set_applet_icon_name("dialog-warning-symbolic");
        this.set_applet_label("Res ?");
        this.set_applet_tooltip("Remote Studio\n" + reason + "\nStatus: " + STATUS_FILE);
    },

    _readStatus: function() {
        try {
            let [ok, buf] = GLib.file_get_contents(STATUS_FILE);
            if (!ok) return;
            let status = this._parseStatus(buf.toString());
            if (!status) {
                this._setUnavailable("Status data is incomplete");
                return;
            }

            let users = status.users;
            let notify = this._notifyEnabled !== false;

            if (notify) {
                if (users > this.lastUserCount)
                    Util.spawn(["notify-send", "-u", "critical", "Remote Studio",
                        (users - this.lastUserCount) + " user(s) connected"]);
                else if (users < this.lastUserCount && this.lastUserCount > 0)
                    Util.spawn(["notify-send", "-u", "normal", "Remote Studio",
                        "User disconnected (" + users + " remaining)"]);
            }
            this.lastUserCount = users;
            this._lastStatus = status;

            this.set_applet_icon_name(this._iconForMode(status.label));
            this.set_applet_label(this._panelLabel(status));
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
                "\nTraffic: " + status.traffic    +
                "\nWarnings: "+ status.warningText
            );

            if (this.menu && this.menu.isOpen) this._buildMenu();
        } catch(e) {
            this._setUnavailable("Status read failed");
        }
    },

    _loadProfiles: function() {
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
        return order.map(k => seen[k]);
    },

    _runRes: function(arg) {
        Util.spawn([RES_CMD].concat(arg.split(" ")));
        GLib.timeout_add(GLib.PRIORITY_DEFAULT, 1200, () => {
            this._scheduleUpdate();
            this._buildMenu();
            return GLib.SOURCE_REMOVE;
        });
    },

    _makeGroup: function(title, stateKey) {
        let group = new PopupMenu.PopupSubMenuMenuItem(title);
        if (_groupOpen[stateKey]) group.menu.open(false);
        group.menu.connect("open-state-changed", (menu, open) => {
            _groupOpen[stateKey] = open;
        });
        return group;
    },

    _subHeader: function(parent, label) {
        let item = new PopupMenu.PopupMenuItem(label, {
            reactive: false,
            style_class: "popup-subtitle-menu-item"
        });
        parent.addMenuItem(item);
        return item;
    },

    _subItem: function(group, label, icon, arg, tooltip) {
        let item = new PopupMenu.PopupIconMenuItem(label, icon, St.IconType.SYMBOLIC);
        if (tooltip && item.actor && item.actor.set_tooltip_text)
            item.actor.set_tooltip_text(tooltip);
        item.connect("activate", () => this._runRes(arg));
        group.menu.addMenuItem(item);
        return item;
    },

    _subTerminal: function(group, label, icon, arg, tooltip) {
        let item = new PopupMenu.PopupIconMenuItem(label, icon, St.IconType.SYMBOLIC);
        if (tooltip && item.actor && item.actor.set_tooltip_text)
            item.actor.set_tooltip_text(tooltip);
        item.connect("activate", () => {
            Util.spawn(["x-terminal-emulator", "-e", RES_CMD].concat(arg.split(" ")));
        });
        group.menu.addMenuItem(item);
    },

    _subSep: function(group) {
        group.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());
    },

    _addTextScalingSlider: function(group) {
        this._subSep(group);
        this._subHeader(group.menu, "Text Scaling");

        let slider = new PopupMenu.PopupSliderMenuItem(0);
        if (slider.actor && slider.actor.set_tooltip_text)
            slider.actor.set_tooltip_text("Adjust Cinnamon desktop text scaling (0.5\u20132.0)");

        try {
            let proc = Gio.Subprocess.new(
                ["gsettings", "get", "org.cinnamon.desktop.interface", "text-scaling-factor"],
                Gio.SubprocessFlags.STDOUT_PIPE | Gio.SubprocessFlags.STDERR_PIPE
            );
            proc.communicate_utf8_async(null, null, (p, res) => {
                try {
                    let [ok, out] = p.communicate_utf8_finish(res);
                    if (ok && out) {
                        let currentScale = parseFloat(out.trim());
                        if (!isNaN(currentScale)) {
                            let val = (currentScale - 0.5) / 1.5;
                            slider.setValue(Math.max(0, Math.min(1, val)));
                        }
                    }
                } catch(e) {}
            });
        } catch(e) {}

        slider.connect("value-changed", (s, value) => {
            let scale = 0.5 + (value * 1.5);
            Util.spawn(["gsettings", "set", "org.cinnamon.desktop.interface",
                "text-scaling-factor", scale.toFixed(2)]);
        });
        group.menu.addMenuItem(slider);
    },

    _buildStatusHeader: function() {
        let status = this._lastStatus;
        if (!status) return;

        let conn = status.users > 0
            ? (status.connType === "Direct" ? "Direct" : "Relayed") + " \u00d7" + status.users
            : "No active session";
        let detail = status.resolution + (status.codec ? " \u00b7 " + status.codec : "");
        let summary = conn + "  \u00b7  " + detail;

        this._subHeader(this.menu, "LIVE STATUS");
        let statusItem = new PopupMenu.PopupMenuItem(summary, { reactive: false });
        if (statusItem.label) statusItem.label.clutter_text.ellipsize = 3;
        this.menu.addMenuItem(statusItem);

        if (status.users > 0 && status.ip && status.ip !== "N/A" && status.ip !== "None") {
            status.ip.split(",").forEach(ip => {
                ip = ip.trim();
                if (!ip) return;
                let peer = new PopupMenu.PopupIconMenuItem(ip, "avatar-default-symbolic", St.IconType.SYMBOLIC);
                peer.actor.reactive = false;
                this.menu.addMenuItem(peer);
            });
        }

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());
    },

    _buildMenu: function() {
        this.menu.removeAll();

        this._buildStatusHeader();

        let currentMode    = this._getCurrentMode();
        let profiles       = this._loadProfiles();
        let defaultProfile = this._getDefaultProfile();
        let directAddr     = this._lastStatus ? this._lastStatus.direct : null;

        let activeIsInProfiles = profiles.some(p => p.label === currentMode);
        if (activeIsInProfiles) _groupOpen.presets = true;

        let presetsGroup = this._makeGroup("Device Presets", "presets");
        profiles.forEach(p => {
            let active = (currentMode === p.label);
            let mark   = active ? "\u2713 " : "   ";
            let icon   = this._iconForMode(p.label);
            let item   = new PopupMenu.PopupIconMenuItem(
                mark + p.label + "  " + p.width + "\u00d7" + p.height,
                icon, St.IconType.SYMBOLIC
            );
            if (active) item.setShowDot(true);
            if (item.actor && item.actor.set_tooltip_text)
                item.actor.set_tooltip_text("Switch display to " + p.label);
            item.connect("activate", () => this._runRes(p.key));
            presetsGroup.menu.addMenuItem(item);
        });
        this.menu.addMenuItem(presetsGroup);

        let perfGroup = this._makeGroup("Performance & Session", "performance");
        this._subItem(perfGroup, "Start Session (" + defaultProfile + ")",
            "media-playback-start-symbolic", "session start " + defaultProfile,
            "Apply default profile and start an optimized session");
        this._subItem(perfGroup, "Stop Session",
            "media-playback-stop-symbolic", "session stop",
            "End active session and restore layout");
        this._subSep(perfGroup);
        this._subItem(perfGroup, "Toggle Speed Mode",  "go-jump-symbolic", "speed",
            "Optimize for low-latency remote control");
        this._subItem(perfGroup, "Toggle Caffeine",    "battery-symbolic", "caf",
            "Keep display awake permanently");
        this._subItem(perfGroup, "Toggle Theme",       "weather-clear-night-symbolic", "theme",
            "Invert colors for OLED remote clients");
        this._subItem(perfGroup, "Toggle Night Shift", "night-light-symbolic", "night",
            "Reduce blue light for evening sessions");
        this._addTextScalingSlider(perfGroup);
        this.menu.addMenuItem(perfGroup);

        let rdGroup = this._makeGroup("RustDesk", "rustdesk");
        this._subItem(rdGroup, "Quality Preset",  "video-display-symbolic", "rustdesk apply quality",
            "Best image quality, higher bandwidth");
        this._subItem(rdGroup, "Balanced Preset", "video-display-symbolic", "rustdesk apply balanced",
            "Balance quality and latency");
        this._subItem(rdGroup, "Speed Preset",    "video-display-symbolic", "rustdesk apply speed",
            "Lowest latency, reduced quality");
        this._subSep(rdGroup);
        this._subItem(rdGroup, "Restart Service", "network-server-symbolic", "service",
            "Restart the RustDesk background service");
        this.menu.addMenuItem(rdGroup);

        let sysGroup = this._makeGroup("System & Security", "system");
        this._subItem(sysGroup,    "Privacy Shield (Lock)",        "system-lock-screen-symbolic",  "privacy",
            "Lock screen and hide desktop contents");
        this._subItem(sysGroup,    "Fix Clipboard / Audio / Keys", "applications-system-symbolic",  "fix",
            "Repair common remote session issues");
        this._subTerminal(sysGroup,"Run Doctor",                   "dialog-information-symbolic",   "doctor",
            "Run full system diagnostics");
        this._subTerminal(sysGroup,"Show Tailnet Address",         "network-vpn-symbolic",          "tailnet",
            "Display Tailscale network address");
        this._subTerminal(sysGroup,"Tailnet Doctor",               "network-wired-symbolic",        "tailnet doctor",
            "Diagnose Tailscale connectivity");
        this._subItem(sysGroup,    "Emergency Display Reset",      "view-refresh-symbolic",         "reset",
            "Reset display to safe 1024\u00d7768 mode");
        this.menu.addMenuItem(sysGroup);

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        if (directAddr) {
            let copyItem = new PopupMenu.PopupIconMenuItem(
                "Copy Direct Address (" + directAddr + ")",
                "edit-copy-symbolic", St.IconType.SYMBOLIC
            );
            copyItem.connect("activate", () => {
                Util.spawn(["bash", "-c",
                    "printf %s \"$1\" | xclip -selection clipboard",
                    "remote-studio", directAddr]);
            });
            this.menu.addMenuItem(copyItem);
        }

        let muteLabel = this._notifyEnabled !== false
            ? "Mute Connect/Disconnect Alerts"
            : "Unmute Connect/Disconnect Alerts";
        let muteIcon  = this._notifyEnabled !== false
            ? "notifications-disabled-symbolic"
            : "notification-symbolic";
        let muteItem  = new PopupMenu.PopupIconMenuItem(muteLabel, muteIcon, St.IconType.SYMBOLIC);
        muteItem.connect("activate", () => {
            this._notifyEnabled = !this._notifyEnabled;
            this.settings.setValue("notify_enabled", this._notifyEnabled);
            this._buildMenu();
        });
        this.menu.addMenuItem(muteItem);

        let tuiItem = new PopupMenu.PopupIconMenuItem(
            "Open Full TUI Dashboard", "utilities-terminal-symbolic", St.IconType.SYMBOLIC
        );
        tuiItem.connect("activate", () => { Util.spawn(["x-terminal-emulator", "-e", RES_CMD]); });
        this.menu.addMenuItem(tuiItem);
    },

    on_applet_clicked: function(event) {
        this._readStatus();
        this._buildMenu();
        this._scheduleUpdate();
        this.menu.toggle();
    },

    on_applet_removed_from_panel: function() {
        if (this._timeout)     GLib.source_remove(this._timeout);
        if (this._fileMonitor) this._fileMonitor.cancel();
    },
};

function main(metadata, orientation, panel_height, instance_id) {
    return new MyApplet(metadata, orientation, panel_height, instance_id);
}
