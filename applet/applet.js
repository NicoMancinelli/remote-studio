const Applet = imports.ui.applet;
const PopupMenu = imports.ui.popupMenu;
const Util = imports.misc.util;
const GLib = imports.gi.GLib;
const Mainloop = imports.mainloop;
const St = imports.gi.St;

const RES_CMD = "/usr/local/bin/res";
const RUNTIME_DIR = GLib.getenv("XDG_RUNTIME_DIR") || "/tmp";
const STATUS_DIR = GLib.file_test(RUNTIME_DIR, GLib.FileTest.IS_DIR) ? RUNTIME_DIR + "/remote-studio" : "/tmp/remote-studio";
const STATUS_FILE = STATUS_DIR + "/status";
const STATE_FILE = GLib.get_home_dir() + "/.res_state";
const ROOT_DIR = GLib.path_get_dirname(GLib.file_read_link(RES_CMD, null) || RES_CMD);
const PROFILES_FILE = ROOT_DIR + "/config/profiles.conf";
const USER_PROFILES_FILE = GLib.get_home_dir() + "/.config/remote-studio/profiles.conf";

// Map mode names to panel icons
const MODE_ICONS = {
    "MacBook Air 13": "computer-symbolic",
    "MacBook Air 15": "computer-symbolic",
    "iPad Pro 11\"": "tablet-symbolic",
    "iPhone Landscape": "phone-symbolic",
    "iPhone Portrait": "phone-symbolic",
    "Reset": "view-refresh-symbolic"
};

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

        this.menuManager = new PopupMenu.PopupMenuManager(this);
        this.menu = new Applet.AppletPopupMenu(this, orientation);
        this.menuManager.addMenu(this.menu);

        this.lastUserCount = 0;
        this._timeout = null;
        this._readTimeout = null;
        this._scheduleUpdate();
        this._buildMenu();
    },

    _scheduleUpdate: function() {
        if (this._timeout) Mainloop.source_remove(this._timeout);
        if (this._readTimeout) Mainloop.source_remove(this._readTimeout);

        GLib.spawn_command_line_async("/bin/bash -c \"mkdir -p " + STATUS_DIR + " && " + RES_CMD + " status > " + STATUS_FILE + "\"");

        this._readTimeout = Mainloop.timeout_add(600, () => {
            this._readStatus();
            this._readTimeout = null;
            return false;
        });

        this._timeout = Mainloop.timeout_add_seconds(10, () => {
            this._timeout = null;
            this._scheduleUpdate();
            return false;
        });
    },

    _getCurrentMode: function() {
        try {
            let [res, contents] = GLib.file_get_contents(STATE_FILE);
            if (res) {
                let parts = contents.toString().trim().match(/'([^']+)'/);
                return parts ? parts[1] : "";
            }
        } catch(e) {}
        return "";
    },

    _updateIcon: function(modeName) {
        let icon = "video-display-symbolic";
        for (let key in MODE_ICONS) {
            if (modeName.indexOf(key) !== -1) {
                icon = MODE_ICONS[key];
                break;
            }
        }
        if (modeName && modeName.indexOf("Custom") === 0) icon = "preferences-desktop-display-symbolic";
        this.set_applet_icon_name(icon);
    },

    _readStatus: function() {
        try {
            let [res, contents] = GLib.file_get_contents(STATUS_FILE);
            if (!res) return;

            let info = contents.toString().trim().split(" | ");
            if (info.length < 9) return;

            let label = info[0];
            let userCount = parseInt(info[3]) || 0;
            let user_icon = (userCount > 0) ? " \u{1F465}" + userCount : "";
            let warningCount = parseInt(info[5]) || 0;
            let alerts = (warningCount > 0) ? "\u26A0 " : "";

            if (userCount > this.lastUserCount) {
                let diff = userCount - this.lastUserCount;
                Util.spawnCommandLine("notify-send -u critical 'Remote Studio' '" + diff + " user(s) connected'");
            } else if (userCount < this.lastUserCount && this.lastUserCount > 0) {
                Util.spawnCommandLine("notify-send -u normal 'Remote Studio' 'User disconnected (" + userCount + " remaining)'");
            }
            this.lastUserCount = userCount;

            this._updateIcon(label);
            this.set_applet_label(alerts + label + user_icon);
            this.set_applet_tooltip(
                "Remote Studio" +
                "\nMode: " + label +
                "\nTemp: " + info[1] +
                "\nRAM: " + info[4] +
                "\nIP: " + info[8] +
                "\nLatency: " + info[2] +
                "\nTraffic: " + info[7] +
                "\nWarnings: " + info[6]
            );
        } catch(e) {}
    },

    _loadProfiles: function() {
        let profiles = [];
        let files = [PROFILES_FILE, USER_PROFILES_FILE];
        files.forEach(file => {
            try {
                if (GLib.file_test(file, GLib.FileTest.EXISTS)) {
                    let [res, contents] = GLib.file_get_contents(file);
                    if (res) {
                        let lines = contents.toString().split("\n");
                        lines.forEach(line => {
                            if (line && line.indexOf("=") !== -1 && line.indexOf("#") !== 0) {
                                let [key, val] = line.split("=");
                                let [label, w, h, scale] = val.split("|");
                                profiles.push({ key: key.trim(), label: label.trim(), width: w, height: h, scale: scale });
                            }
                        });
                    }
                }
            } catch(e) {}
        });
        return profiles;
    },

    _buildMenu: function() {
        this.menu.removeAll();
        let currentMode = this._getCurrentMode();

        let presetHeader = new PopupMenu.PopupMenuItem("DEVICE PRESETS", { reactive: false, style_class: "popup-subtitle-menu-item" });
        this.menu.addMenuItem(presetHeader);

        let profiles = this._loadProfiles();
        profiles.forEach(p => {
            let isActive = (currentMode === p.label);
            let displayLabel = (isActive ? "\u{2713} " : "   ") + p.label + " (" + p.width + "x" + p.height + ")";
            let icon = "video-display-symbolic";
            for (let k in MODE_ICONS) { if (p.label.indexOf(k) !== -1) { icon = MODE_ICONS[k]; break; } }
            this._addMenuItem(displayLabel, icon, p.key);
        });

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        let perfHeader = new PopupMenu.PopupMenuItem("PERFORMANCE & COMFORT", { reactive: false, style_class: "popup-subtitle-menu-item" });
        this.menu.addMenuItem(perfHeader);
        this._addMenuItem("Toggle Performance Mode", "go-jump-symbolic", "speed");
        this._addMenuItem("Start Mac Session", "media-playback-start-symbolic", "session start mac");
        this._addMenuItem("Stop Session", "media-playback-stop-symbolic", "session stop");
        this._addMenuItem("Toggle OLED Theme", "weather-clear-night-symbolic", "theme");
        this._addMenuItem("Toggle Night Shift", "night-light-symbolic", "night");
        this._addMenuItem("Toggle Caffeine", "battery-symbolic", "caf");

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        let rustdeskHeader = new PopupMenu.PopupMenuItem("RUSTDESK PRESETS", { reactive: false, style_class: "popup-subtitle-menu-item" });
        this.menu.addMenuItem(rustdeskHeader);
        this._addMenuItem("Apply Quality Preset", "video-display-symbolic", "rustdesk apply quality");
        this._addMenuItem("Apply Balanced Preset", "video-display-symbolic", "rustdesk apply balanced");
        this._addMenuItem("Apply Speed Preset", "video-display-symbolic", "rustdesk apply speed");

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        let systemHeader = new PopupMenu.PopupMenuItem("SYSTEM & SECURITY", { reactive: false, style_class: "popup-subtitle-menu-item" });
        this.menu.addMenuItem(systemHeader);
        this._addMenuItem("Privacy Shield (Lock)", "system-lock-screen-symbolic", "privacy");
        this._addMenuItem("Fix Clipboard / Audio / Keys", "applications-system-symbolic", "fix");
        this._addMenuItem("Restart RustDesk Service", "network-server-symbolic", "service");
        this._addTerminalItem("Run Doctor", "dialog-information-symbolic", "doctor");
        this._addTerminalItem("Show Tailnet Address", "network-vpn-symbolic", "tailnet");
        this._addTerminalItem("Tailnet Doctor", "network-wired-symbolic", "tailnet doctor");
        this._addMenuItem("Standard Reset (1024x768)", "view-refresh-symbolic", "reset");

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        let studioItem = new PopupMenu.PopupIconMenuItem("Open Full TUI Dashboard", "utilities-terminal-symbolic", St.IconType.SYMBOLIC);
        studioItem.connect('activate', () => { Util.spawnCommandLine("x-terminal-emulator -e " + RES_CMD); });
        this.menu.addMenuItem(studioItem);
    },

    _addMenuItem: function(label, icon, arg) {
        let item = new PopupMenu.PopupIconMenuItem(label, icon, St.IconType.SYMBOLIC);
        item.connect('activate', () => {
            Util.spawnCommandLine(RES_CMD + " " + arg);
            Mainloop.timeout_add(1000, () => {
                this._scheduleUpdate();
                this._buildMenu();
                return false;
            });
        });
        this.menu.addMenuItem(item);
    },

    _addTerminalItem: function(label, icon, arg) {
        let item = new PopupMenu.PopupIconMenuItem(label, icon, St.IconType.SYMBOLIC);
        item.connect('activate', () => {
            Util.spawnCommandLine("x-terminal-emulator -e " + RES_CMD + " " + arg);
        });
        this.menu.addMenuItem(item);
    },

    on_applet_clicked: function(event) {
        this._buildMenu();
        this._scheduleUpdate();
        this.menu.toggle();
    },

    on_applet_removed_from_panel: function() {
        if (this._timeout) Mainloop.source_remove(this._timeout);
        if (this._readTimeout) Mainloop.source_remove(this._readTimeout);
    }
};

function main(metadata, orientation, panel_height, instance_id) {
    return new MyApplet(metadata, orientation, panel_height, instance_id);
}
