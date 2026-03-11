const Applet = imports.ui.applet;
const PopupMenu = imports.ui.popupMenu;
const Util = imports.misc.util;
const GLib = imports.gi.GLib;
const Mainloop = imports.mainloop;
const St = imports.gi.St;

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
        this._updateLoop();
        this._buildMenu();
    },

    _updateLoop: function() {
        GLib.spawn_command_line_async("/bin/bash -c \"/usr/local/bin/res status > /tmp/res_status\"");
        
        Mainloop.timeout_add(600, () => {
            let [res, contents] = GLib.file_get_contents("/tmp/res_status");
            if (res) {
                let info = contents.toString().trim().split(" | ");
                // INDEX: 0:Mode, 1:Temp, 2:Ping, 3:Users, 4:RAM, 5:Alerts, 6:Traffic, 7:IP
                let label = info[0].replace("[", "").replace("]", "");
                let userCount = parseInt(info[3]);
                let user_icon = (userCount > 0) ? " 👥" + userCount : "";
                let alerts = info[5] || "";
                
                if (userCount > this.lastUserCount) {
                    Util.spawnCommandLine("notify-send -u critical 'Remote Studio' 'User Connected!'");
                }
                this.lastUserCount = userCount;

                this.set_applet_label(alerts + label + user_icon);
                this.set_applet_tooltip("Remote Studio\nIP: " + info[7] + "\nLatency: " + info[2] + "\nTraffic: " + info[6]);
            }
            return false;
        });

        this._timeout = Mainloop.timeout_add_seconds(10, () => this._updateLoop());
    },

    _buildMenu: function() {
        this.menu.removeAll();

        // --- SECTION: DEVICE PRESETS ---
        let presetHeader = new PopupMenu.PopupMenuItem("--- [ DEVICE PRESETS ] ---", { reactive: false });
        this.menu.addMenuItem(presetHeader);
        this._addMenuItem("MacBook Air (16:10)", "computer-symbolic", "mac");
        this._addMenuItem("iPad Pro 11\" (3:2)", "tablet-symbolic", "ipad");
        this._addMenuItem("iPhone Landscape", "phone-symbolic", "iphonel");
        this._addMenuItem("iPhone Portrait", "phone-symbolic", "iphonep");

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        // --- SECTION: PERFORMANCE & COMFORT ---
        let perfHeader = new PopupMenu.PopupMenuItem("--- [ PERFORMANCE & COMFORT ] ---", { reactive: false });
        this.menu.addMenuItem(perfHeader);
        this._addMenuItem("Toggle Performance Mode", "go-jump-symbolic", "speed");
        this._addMenuItem("Toggle OLED Theme 🌙", "weather-clear-night-symbolic", "theme");
        this._addMenuItem("Toggle Night Shift", "night-light-symbolic", "night");
        this._addMenuItem("Toggle Caffeine", "battery-symbolic", "caf");

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        // --- SECTION: SYSTEM & SECURITY ---
        let systemHeader = new PopupMenu.PopupMenuItem("--- [ SYSTEM & SECURITY ] ---", { reactive: false });
        this.menu.addMenuItem(systemHeader);
        this._addMenuItem("Privacy Shield (Lock)", "system-lock-screen-symbolic", "privacy");
        this._addMenuItem("Flush Clipboard Sync", "edit-paste-symbolic", "clip");
        this._addMenuItem("Restart RustDesk Service", "network-server-symbolic", "service");
        this._addMenuItem("Standard Reset (1024x768)", "view-refresh-symbolic", "reset");

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());
        
        let studioItem = new PopupMenu.PopupIconMenuItem("Open Full TUI Dashboard", "utilities-terminal-symbolic", St.IconType.SYMBOLIC);
        studioItem.connect('activate', () => { Util.spawnCommandLine("gnome-terminal -- /usr/local/bin/res"); });
        this.menu.addMenuItem(studioItem);
    },

    _addMenuItem: function(label, icon, arg) {
        let item = new PopupMenu.PopupIconMenuItem(label, icon, St.IconType.SYMBOLIC);
        item.connect('activate', () => {
            Util.spawnCommandLine("/usr/local/bin/res " + arg);
            Mainloop.timeout_add(1000, () => this._updateLoop());
        });
        this.menu.addMenuItem(item);
    },

    on_applet_clicked: function(event) {
        this._updateLoop();
        this.menu.toggle();
    },

    on_applet_removed_from_panel: function() {
        if (this._timeout) Mainloop.source_remove(this._timeout);
    }
};

function main(metadata, orientation, panel_height, instance_id) {
    return new MyApplet(metadata, orientation, panel_height, instance_id);
}
