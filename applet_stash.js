const Applet = imports.ui.applet;
const PopupMenu = imports.ui.popupMenu;
const Settings = imports.ui.settings;
const Util = imports.misc.util;
const GLib = imports.gi.GLib;
const Gio = imports.gi.Gio;
const Mainloop = imports.mainloop;
const St = imports.gi.St;

const RES_CMD = "/usr/local/bin/res";
const RUNTIME_DIR = GLib.getenv("XDG_RUNTIME_DIR") || "/tmp";
const STATUS_DIR = GLib.file_test(RUNTIME_DIR, GLib.FileTest.IS_DIR) ? RUNTIME_DIR + "/remote-studio" : "/tmp/remote-studio";
const STATUS_FILE = STATUS_DIR + "/status";
const STATE_FILE = GLib.get_home_dir() + "/.res_state";

// Resolve the profiles file: follow the res symlink to find the repo's config/,
// then fall back to the .deb install path at /usr/share/remote-studio/.
const _resLink = GLib.file_read_link(RES_CMD, null);
const _repoRoot = _resLink ? GLib.path_get_dirname(_resLink) : null;
const _repoProfiles = _repoRoot ? _repoRoot + "/config/profiles.conf" : null;
const PROFILES_FILE = (_repoProfiles && GLib.file_test(_repoProfiles, GLib.FileTest.EXISTS))
    ? _repoProfiles
    : "/usr/share/remote-studio/profiles.conf";
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
        this._fileMonitor = null;

        // Bind Settings
        this.settings = new Settings.AppletSettings(this, metadata.uuid, instance_id);
        this.settings.bind("default_profile", "defaultProfile", this.on_settings_changed);
        this.settings.bind("auto_session", "autoSession", this.on_settings_changed);
        this.settings.bind("notify_enabled", "_notifyEnabled", this._buildMenu);

        this._readStatus();
        this._setupDBusOrFileWatch();
        this._buildMenu();
    },

    on_settings_changed: function() {
        Util.spawnCommandLine(RES_CMD + " config set DEFAULT_PROFILE " + this.defaultProfile);
        Util.spawnCommandLine(RES_CMD + " config set AUTO_SESSION " + (this.autoSession ? "true" : "false"));
    },

    _setupDBusOrFileWatch: function() {
        try {
            Gio.DBusProxy.new_for_bus(
                Gio.BusType.SESSION,
                Gio.DBusProxyFlags.NONE,
                null,
                "org.remote_studio.Daemon",
                "/org/remote_studio/Daemon",
                "org.remote_studio.Daemon",
                null,
                (src, res) => {
                    try {
                        this._dbusProxy = Gio.DBusProxy.new_for_bus_finish(res);
                        this._dbusProxy.connect("g-signal", (proxy, sender_name, signal_name, parameters) => {
                            if (signal_name === "StatusChanged") {
                                this._readStatus();
                            }
                        });
                    } catch (e) {
                        this._setupFileWatch();
                    }
                }
            );
        } catch(e) {
            this._setupFileWatch();
        }
    },

    _setupFileWatch: function() {
        try {
            let statusFile = Gio.File.new_for_path(STATUS_FILE);
            // Ensure the directory exists
            let statusDir = Gio.File.new_for_path(STATUS_DIR);
            if (!statusDir.query_exists(null)) {
                statusDir.make_directory_with_parents(null);
            }
            // Create an empty status file if it doesn't exist so we can watch it
            if (!statusFile.query_exists(null)) {
                statusFile.replace_contents("", null, false, Gio.FileCreateFlags.NONE, null);
            }
            this._fileMonitor = statusFile.monitor_file(Gio.FileMonitorFlags.NONE, null);
            this._fileMonitor.connect('changed', (monitor, file, other, eventType) => {
                if (eventType === Gio.FileMonitorEvent.CHANGES_DONE_HINT ||
                    eventType === Gio.FileMonitorEvent.MODIFIED) {
                    this._readStatus();
                }
            });
        } catch(e) {
            // Fall back to polling if file monitoring fails
            global.logError("Remote Studio: file watch setup failed: " + e);
        }
    },

    _getCurrentMode: function() {
        try {
            let [res, contents] = GLib.file_get_contents(STATE_FILE);
            if (res) {
                let parts = contents.toString().trim().match(/'([^']+)'/);
                return parts ? parts[1] : "";
            }
        } catch(e) {
            global.logError("Remote Studio: _getCurrentMode: " + e);
        }
        return "";
    },

    _updateIcon: function(modeName) {
        if (this.lastUserCount > 0) {
            this.set_applet_icon_name("network-wired-symbolic");
            return;
        }
        let icon = "preferences-desktop-display"; // Default icon
        for (let key in MODE_ICONS) {
            if (modeName && modeName.indexOf(key) !== -1) {
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
            let alerts = (warningCount > 0) ? "⚠ " : "";
            let connType = (info[9] || "").trim();

            if (this._notifyEnabled) {
                if (userCount > this.lastUserCount) {
                    let diff = userCount - this.lastUserCount;
                    Util.spawnCommandLine("notify-send -u critical 'Remote Studio' '" + diff + " user(s) connected'");
                } else if (userCount < this.lastUserCount && this.lastUserCount > 0) {
                    Util.spawnCommandLine("notify-send -u normal 'Remote Studio' 'User disconnected (" + userCount + " remaining)'");
                }
            }
            this.lastUserCount = userCount;
            this._activeIPs = (info[8] || "").trim();
            this._directAddress = (info[11] || "").trim();

            let qualityDot = "";
            if (userCount > 0) {
                qualityDot = (connType === "Direct") ? "● " : "◐ ";
            }

            this._updateIcon(label);
            this.set_applet_label(alerts + qualityDot + label + user_icon);
            let codec = (info[12] || "").trim();
            let fps = (info[13] || "").trim();

            let tooltipStr = "Remote Studio" +
                "\nMode: " + label +
                "\nRes: " + (info[10] || "N/A") +
                "\nPath: " + (info[9] || "N/A");
                
            if (codec && codec !== "N/A") {
                tooltipStr += "\nCodec/FPS: " + codec + " @ " + fps;
            }

            tooltipStr += "\nIP: " + info[8] +
                "\nDirect: " + (info[11] || "N/A") +
                "\nTemp: " + info[1] +
                "\nRAM: " + info[4] +
                "\nLatency: " + info[2] +
                "\nTraffic: " + info[7] +
                "\nWarnings: " + info[6];

            this.set_applet_tooltip(tooltipStr);

            if (this.menu && this.menu.isOpen) {
                this._buildMenu();
            }
        } catch(e) {
            global.logError("Remote Studio: _readStatus: " + e);
        }
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
            } catch(e) {
                global.logError("Remote Studio: _loadProfiles: " + e);
            }
        });
        return profiles;
    },

    _buildMenu: function() {
        this.menu.removeAll();
        let currentMode = this._getCurrentMode();

        if (this.lastUserCount > 0 && this._activeIPs && this._activeIPs !== "N/A" && this._activeIPs !== "None" && this._activeIPs !== "") {
            let usersHeader = new PopupMenu.PopupMenuItem("ACTIVE SESSIONS", { reactive: false, style_class: "popup-subtitle-menu-item" });
            this.menu.addMenuItem(usersHeader);
            
            let ips = this._activeIPs.split(",");
            ips.forEach(ip => {
                ip = ip.trim();
                if (ip) {
                    let item = new PopupMenu.PopupIconMenuItem(ip, "avatar-default-symbolic", St.IconType.SYMBOLIC);
                    this.menu.addMenuItem(item);
                }
            });
            this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());
        }

        let presetHeader = new PopupMenu.PopupMenuItem("DEVICE PRESETS", { reactive: false, style_class: "popup-subtitle-menu-item" });
        this.menu.addMenuItem(presetHeader);

        let profiles = this._loadProfiles();
        profiles.forEach(p => {
            let isActive = (currentMode === p.label);
            let displayLabel = (isActive ? "\u{2713} " : "   ") + p.label + " (" + p.width + "x" + p.height + ")";
            let icon = "video-display-symbolic";
            for (let k in MODE_ICONS) { if (p.label.indexOf(k) !== -1) { icon = MODE_ICONS[k]; break; } }
            this.menu.addMenuItem(this._createMenuItem(displayLabel, icon, p.key, false, "Switch resolution to perfectly match a " + p.label));
        });

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        let perfMenu = new PopupMenu.PopupSubMenuMenuItem("Performance & Comfort");
        perfMenu.menu.addMenuItem(this._createMenuItem("Toggle Performance Mode", "go-jump-symbolic", "speed", false, "Optimize for low-latency speed"));
        perfMenu.menu.addMenuItem(this._createMenuItem("Start Mac Session", "media-playback-start-symbolic", "session start mac", false, "Start a Mac-optimized session"));
        perfMenu.menu.addMenuItem(this._createMenuItem("Stop Session", "media-playback-stop-symbolic", "session stop", false, "End active session and restore layout"));
        perfMenu.menu.addMenuItem(this._createMenuItem("Toggle OLED Theme", "weather-clear-night-symbolic", "theme", false, "Invert colors for OLED screens to save power"));
        perfMenu.menu.addMenuItem(this._createMenuItem("Toggle Night Shift", "night-light-symbolic", "night", false, "Reduce blue light for eye comfort at night"));
        perfMenu.menu.addMenuItem(this._createMenuItem("Toggle Caffeine", "battery-symbolic", "caf", false, "Keep display awake permanently"));

        // UI Scaling Slider
        perfMenu.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());
        let sliderLabel = new PopupMenu.PopupMenuItem("Text Scaling", { reactive: false, style_class: "popup-subtitle-menu-item" });
        perfMenu.menu.addMenuItem(sliderLabel);
        
        let slider = new PopupMenu.PopupSliderMenuItem(0);
        if (slider.actor && slider.actor.set_tooltip_text) {
            slider.actor.set_tooltip_text("Slide to instantly adjust the size of Cinnamon desktop fonts and UI elements");
        }
        // Default scale factor bounds (0.5 to 2.0). Map [0, 1] -> [0.5, 2.0]
        try {
            let proc = Gio.Subprocess.new(
                ['gsettings', 'get', 'org.cinnamon.desktop.interface', 'text-scaling-factor'],
                Gio.SubprocessFlags.STDOUT_PIPE | Gio.SubprocessFlags.STDERR_PIPE
            );
            proc.communicate_utf8_async(null, null, (p, res) => {
                try {
                    let [ok, out, err] = p.communicate_utf8_finish(res);
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
        
        slider.connect('value-changed', (slider, value) => {
            let scale = 0.5 + (value * 1.5);
            Util.spawnCommandLine("gsettings set org.cinnamon.desktop.interface text-scaling-factor " + scale.toFixed(2));
        });
        perfMenu.menu.addMenuItem(slider);

        this.menu.addMenuItem(perfMenu);

        let rustdeskMenu = new PopupMenu.PopupSubMenuMenuItem("RustDesk Presets");
        rustdeskMenu.menu.addMenuItem(this._createMenuItem("Apply Quality Preset", "video-display-symbolic", "rustdesk apply quality", false));
        rustdeskMenu.menu.addMenuItem(this._createMenuItem("Apply Balanced Preset", "video-display-symbolic", "rustdesk apply balanced", false));
        rustdeskMenu.menu.addMenuItem(this._createMenuItem("Apply Speed Preset", "video-display-symbolic", "rustdesk apply speed", false));
        this.menu.addMenuItem(rustdeskMenu);

        let systemMenu = new PopupMenu.PopupSubMenuMenuItem("System & Security");
        systemMenu.menu.addMenuItem(this._createMenuItem("Privacy Shield (Lock)", "system-lock-screen-symbolic", "privacy", false));
        systemMenu.menu.addMenuItem(this._createMenuItem("Fix Clipboard / Audio / Keys", "applications-system-symbolic", "fix", false));
        systemMenu.menu.addMenuItem(this._createMenuItem("Restart RustDesk Service", "network-server-symbolic", "service", false));
        systemMenu.menu.addMenuItem(this._createMenuItem("Run Doctor", "dialog-information-symbolic", "doctor", true));
        systemMenu.menu.addMenuItem(this._createMenuItem("Show Tailnet Address", "network-vpn-symbolic", "tailnet", true));
        systemMenu.menu.addMenuItem(this._createMenuItem("Tailnet Doctor", "network-wired-symbolic", "tailnet doctor", true));
        systemMenu.menu.addMenuItem(this._createMenuItem("Standard Reset (1024x768)", "view-refresh-symbolic", "reset", false));
        this.menu.addMenuItem(systemMenu);

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        // Copy Direct Address
        let directAddr = this._directAddress;
        if (directAddr && directAddr !== "N/A" && directAddr !== "") {
            let copyItem = new PopupMenu.PopupIconMenuItem(
                "Copy Direct Address (" + directAddr + ")",
                "edit-copy-symbolic",
                St.IconType.SYMBOLIC
            );
            copyItem.connect('activate', () => {
                Util.spawnCommandLine("bash -c \"echo -n " + GLib.shell_quote(directAddr) + " | xclip -selection clipboard\"");
            });
            this.menu.addMenuItem(copyItem);
        }

        // Settings are now handled via the native applet configure dialog


        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        let studioItem = new PopupMenu.PopupIconMenuItem("Open Full TUI Dashboard", "utilities-terminal-symbolic", St.IconType.SYMBOLIC);
        studioItem.connect('activate', () => { Util.spawn(["x-terminal-emulator", "-e", RES_CMD]); });
        this.menu.addMenuItem(studioItem);
    },

    _createMenuItem: function(label, icon, arg, isTerminal, tooltip) {
        let item = new PopupMenu.PopupIconMenuItem(label, icon, St.IconType.SYMBOLIC);
        if (tooltip && item.actor.set_tooltip_text) {
            item.actor.set_tooltip_text(tooltip);
        }
        item.connect('activate', () => {
            if (isTerminal) {
                Util.spawn(["x-terminal-emulator", "-e", RES_CMD].concat(arg.split(" ")));
            } else {
                Util.spawn([RES_CMD].concat(arg.split(" ")));
            }
        });
        return item;
    },

    on_applet_clicked: function(event) {
        this._readStatus();
        this._buildMenu();
        this.menu.toggle();
    },

    on_applet_removed_from_panel: function() {
        if (this._fileMonitor) this._fileMonitor.cancel();
    }
};

function main(metadata, orientation, panel_height, instance_id) {
    return new MyApplet(metadata, orientation, panel_height, instance_id);
}
