#!/usr/bin/env node
// Unit tests for the pure logic in applet/applet.js.
// Loads the applet with mocked GJS globals (imports, global) so its
// status parsing, panel label, and color logic can run under node.
"use strict";

const fs = require("fs");
const path = require("path");
const vm = require("vm");

const APPLET_PATH = path.join(__dirname, "..", "..", "applet", "applet.js");

// ── GJS mocks ─────────────────────────────────────────────────────────────

const noop = () => {};
const gioMock = {
    DBusProxy: { makeProxyWrapper: () => function() {} },
    DBus: { session: { watch_name: () => 1, unwatch_name: noop } },
    File: { new_for_path: () => ({ monitor: () => ({ connect: noop, cancel: noop }) }) },
    FileMonitorFlags: { NONE: 0, WATCH_MOVES: 1 },
    FileMonitorEvent: { CHANGED: 0, CHANGES_DONE_HINT: 1, CREATED: 2 },
    BusNameWatcherFlags: { NONE: 0 },
};
const glibMock = {
    getenv: () => null,
    get_user_runtime_dir: () => "/tmp/mock-runtime",
    get_user_name: () => "mockuser",
    get_home_dir: () => "/tmp/mock-home",
    file_test: () => false,
    file_get_contents: () => { throw new Error("no file"); },
    file_set_contents: noop,
    file_read_link: () => { throw new Error("not a link"); },
    file_delete: noop,
    path_get_dirname: (p) => path.dirname(p),
    mkdir_with_parents: noop,
    find_program_in_path: () => null,
    timeout_add: () => 1,
    timeout_add_seconds: () => 1,
    source_remove: noop,
    FileTest: { EXISTS: 16, IS_DIR: 4 },
    PRIORITY_DEFAULT: 0,
    SOURCE_CONTINUE: true,
    SOURCE_REMOVE: false,
};

const sandbox = {
    imports: {
        ui: {
            applet: { TextIconApplet: { prototype: {} }, AppletPopupMenu: function() {} },
            popupMenu: {
                PopupMenuManager: function() {},
                PopupSubMenuMenuItem: function() {},
                PopupIconMenuItem: function() {},
                PopupSeparatorMenuItem: function() {},
            },
            settings: { AppletSettings: function() {}, BindingDirection: { IN: 0, BIDIRECTIONAL: 2 } },
        },
        misc: { util: { spawn: noop } },
        gi: { GLib: glibMock, Gio: gioMock, St: { IconType: { SYMBOLIC: 0 } } },
    },
    global: { log: noop, logError: noop },
    Date: Date,
    Math: Math,
    JSON: JSON,
    parseInt: parseInt,
    isNaN: isNaN,
    String: String,
    Array: Array,
};
vm.createContext(sandbox);
vm.runInContext(fs.readFileSync(APPLET_PATH, "utf8"), sandbox, { filename: "applet.js" });

// A bare object carrying just the prototype methods — _init is never called.
const proto = sandbox.MyApplet.prototype;
const applet = Object.create(proto);
applet._uptimeDisplay = true;
applet._sessionStart = null;

// ── Tiny test runner ──────────────────────────────────────────────────────

let failures = 0, passed = 0;
function assertEq(actual, expected, label) {
    const okA = JSON.stringify(actual), okE = JSON.stringify(expected);
    if (okA === okE) { passed++; return; }
    failures++;
    console.error(`FAIL: ${label}\n  expected: ${okE}\n  actual:   ${okA}`);
}

// ── _parseStatus: pipe format ─────────────────────────────────────────────

const pipeRaw = "MacBook Air 13 | 42°C | 12ms | 1 | 3.1G | 0 | none | 1.2MB/s | 100.64.0.1 | Direct | 2560×1664 | 100.64.0.1:21118 | av1";
const pipe = applet._parseStatus(pipeRaw);
assertEq(pipe.label, "MacBook Air 13", "pipe: label");
assertEq(pipe.users, 1, "pipe: users");
assertEq(pipe.warnings, 0, "pipe: warnings");
assertEq(pipe.connType, "Direct", "pipe: connType");
assertEq(pipe.direct, "100.64.0.1:21118", "pipe: direct");
assertEq(pipe.codec, "av1", "pipe: codec");
assertEq(applet._parseStatus("too | short"), null, "pipe: short line rejected");
assertEq(applet._parseStatus(""), null, "empty status rejected");
assertEq(applet._parseStatus("A | B | C | 2 | D | 1 | warn | E | F"), {
    label: "A", temp: "B", latency: "C", users: 2, ram: "D", warnings: 1,
    warningText: "warn", traffic: "E", ip: "F", connType: "N/A",
    resolution: "N/A", direct: null, codec: "", active_ips: []
}, "pipe: 9-field minimum with defaults");

// codec "none" is normalized to empty
const pipeNone = applet._parseStatus("M | t | l | 0 | r | 0 | none | tr | ip | Relay | 1024×768 |  | none");
assertEq(pipeNone.codec, "", "pipe: codec 'none' normalized");

// ── _parseStatus: JSON format ─────────────────────────────────────────────

const jsonRaw = JSON.stringify({
    mode: "iPad Pro 13", temperature: "50°C", latency: "25ms", users: "2",
    ram: "4G", warnings: { count: 1, summary: "tailscale down" },
    network: "0.5MB/s", ip: "10.0.0.5", connection: "Relay",
    resolution: "2064×2752", direct_address: " 10.0.0.5:21118 ",
    codec: "vp9", active_ips: ["10.0.0.9"]
});
const js = applet._parseStatus(jsonRaw);
assertEq(js.label, "iPad Pro 13", "json: label");
assertEq(js.users, 2, "json: users coerced to int");
assertEq(js.warnings, 1, "json: warnings count");
assertEq(js.warningText, "tailscale down", "json: warning summary");
assertEq(js.direct, "10.0.0.5:21118", "json: direct trimmed");
assertEq(js.active_ips, ["10.0.0.9"], "json: active_ips");
assertEq(applet._parseStatus("{not json"), null, "json: invalid rejected");

// ── _ellipsize / _compactModeLabel ────────────────────────────────────────

assertEq(applet._ellipsize("short", 10), "short", "ellipsize: no-op");
assertEq(applet._ellipsize("abcdefghij", 5), "abcd…", "ellipsize: truncates");
assertEq(applet._compactModeLabel("iPad Pro 13 (2064x2752 Retina)"), "iPad Pro 13", "compact: parens stripped");
assertEq(applet._compactModeLabel(null), "Unknown", "compact: null → Unknown");

// ── _formatUptime ─────────────────────────────────────────────────────────

assertEq(applet._formatUptime(0), "", "uptime: zero");
assertEq(applet._formatUptime(59), "59s", "uptime: seconds");
assertEq(applet._formatUptime(125), "2m 5s", "uptime: minutes");
assertEq(applet._formatUptime(3720), "1h 2m", "uptime: hours");

// ── _panelColorForStatus ──────────────────────────────────────────────────

assertEq(applet._panelColorForStatus(null), "#ff6600", "color: null → orange");
assertEq(applet._panelColorForStatus({ warnings: 1, users: 0, label: "X" }), "#ffaa00", "color: warnings → yellow");
assertEq(applet._panelColorForStatus({ warnings: 0, users: 1, connType: "Direct" }), "#00cc66", "color: direct → green");
assertEq(applet._panelColorForStatus({ warnings: 0, users: 1, connType: "Relay" }), "#66bbff", "color: relay → blue");
assertEq(applet._panelColorForStatus({ warnings: 0, users: 0, label: "MacBook Air 13" }), "#aaaaaa", "color: idle session → gray");
assertEq(applet._panelColorForStatus({ warnings: 0, users: 0, label: "Unknown" }), "#888888", "color: no session → dim");

// ── _panelLabel ───────────────────────────────────────────────────────────

const MAX = 28;
assertEq(applet._panelLabel({ users: 0, warnings: 0, connType: "N/A", label: "MacBook Air 13" }),
    "MacBook Air 13", "label: plain");
assertEq(applet._panelLabel({ users: 1, warnings: 0, connType: "Direct", label: "MacBook Air 13" }),
    "● MacBook Air 13 👥1", "label: direct user dot");
assertEq(applet._panelLabel({ users: 1, warnings: 1, connType: "Relay", label: "MacBook Air 13" }),
    "⚠ ◐ MacBook Air 13 👥1", "label: warning + relay dot");
const longLabel = applet._panelLabel({ users: 2, warnings: 1, connType: "Direct",
    label: "Extremely Long Device Profile Name (3840x2160)" });
assertEq(longLabel.length <= MAX, true, "label: stays within MAX_PANEL_LABEL");

// ── _iconForMode ──────────────────────────────────────────────────────────

assertEq(applet._iconForMode("iPad Pro 13"), "tablet-symbolic", "icon: ipad");
assertEq(applet._iconForMode("iPhone Portrait"), "phone-symbolic", "icon: iphone");
assertEq(applet._iconForMode("Custom 1920x1080"), "preferences-desktop-display-symbolic", "icon: custom");
assertEq(applet._iconForMode("Something Else"), "video-display-symbolic", "icon: fallback");

// ── result ────────────────────────────────────────────────────────────────

if (failures > 0) {
    console.error(`\n${failures} test(s) failed, ${passed} passed`);
    process.exit(1);
}
console.log(`applet tests: ${passed} passed`);
