package display

// ---------------------------------------------------------------------------
// GNOME Shell Extension integration (future — scaffold only)
// ---------------------------------------------------------------------------
//
// This file is a placeholder for direct GNOME Shell Extension integration
// over DBus. When implemented it will allow Remote Studio to control display
// settings on GNOME/Wayland without requiring external CLI tools like
// gnome-randr.
//
// === Architecture ===
//
// GNOME Shell exposes extensions via the D-Bus interface:
//
//   Bus:        org.gnome.Shell
//   Object:     /org/gnome/Shell
//   Interface:  org.gnome.Shell.Extensions
//
// Relevant methods:
//   - ListExtensions() → a{sa{sv}}    — lists installed extensions
//   - GetExtensionInfo(uuid string)    — metadata for a single extension
//   - EnableExtension(uuid string)     — enable an extension by UUID
//   - DisableExtension(uuid string)    — disable an extension by UUID
//   - InstallRemoteExtension(uuid)     — trigger installation from e.g.o
//
// For display management the planned approach is:
//
// 1. Ship a small GNOME Shell extension (e.g. "remote-studio@gnome.ext")
//    that exposes a private DBus interface for:
//      - Setting resolution  (via Meta.MonitorManager / DisplayConfig)
//      - Setting scale       (via org.gnome.Mutter.DisplayConfig)
//      - Rotating outputs
//      - Listing outputs
//
// 2. Connect to that extension's DBus interface from this Go package using
//    github.com/godbus/dbus/v5 (already in go.mod).
//
// 3. Fall back to gnome-randr CLI if the extension is not installed.
//
// === How to wire it up ===
//
// Use the godbus library to call into org.gnome.Mutter.DisplayConfig:
//
//   conn, err := dbus.ConnectSessionBus()
//   obj := conn.Object("org.gnome.Mutter.DisplayConfig",
//                       "/org/gnome/Mutter/DisplayConfig")
//
//   // GetResources returns the full display topology.
//   call := obj.Call("org.gnome.Mutter.DisplayConfig.GetResources", 0)
//
// The GetResources reply contains:
//   serial:     uint32
//   crtcs:      a(uxiiiiiuaua{sv})
//   outputs:    a(uxia{sv}au)
//   modes:      a(uxuud)
//   max_width:  int32
//   max_height: int32
//
// To apply changes use ApplyMonitorsConfig (GNOME 44+):
//   obj.Call("org.gnome.Mutter.DisplayConfig.ApplyMonitorsConfig",
//            0, serial, method, logicalMonitors, properties)
//
// Where method is:
//   1 = verify, 2 = temporary (with 20s revert timeout), 3 = persistent
//
// === Future implementation steps ===
//
// When this module is implemented the display.go dispatcher should call
// gnomeExtSetResolution / gnomeExtListOutputs / etc. as the preferred
// Wayland/GNOME path, with the CLI tools as fallback.
// ---------------------------------------------------------------------------

// GNOMEExtensionUUID is the planned UUID for the companion GNOME Shell
// extension that will be used for native display management over DBus.
const GNOMEExtensionUUID = "remote-studio@gnome.ext"

// GNOMEExtensionAvailable checks whether the Remote Studio GNOME Shell
// extension is installed and enabled. Returns false until the extension
// is actually implemented and shipped.
//
// Future implementation will use:
//
//	conn, _ := dbus.ConnectSessionBus()
//	obj := conn.Object("org.gnome.Shell", "/org/gnome/Shell")
//	var info map[string]map[string]dbus.Variant
//	obj.Call("org.gnome.Shell.Extensions.GetExtensionInfo", 0, GNOMEExtensionUUID).Store(&info)
func GNOMEExtensionAvailable() bool {
	// TODO: implement DBus probe once the GNOME extension is shipped.
	return false
}

// gnomeExtSetResolution is a placeholder for setting resolution via the
// GNOME Shell extension's DBus interface.
//
//nolint:unused // scaffold — will be called from display.go once implemented
func gnomeExtSetResolution(width, height int) error {
	_ = width
	_ = height
	// TODO: Call org.gnome.Mutter.DisplayConfig.ApplyMonitorsConfig
	return nil
}

// gnomeExtSetScale is a placeholder for setting the scale factor via the
// GNOME Shell extension's DBus interface.
//
//nolint:unused // scaffold — will be called from display.go once implemented
func gnomeExtSetScale(factor float64) error {
	_ = factor
	// TODO: Call org.gnome.Mutter.DisplayConfig.ApplyMonitorsConfig with scale
	return nil
}

// gnomeExtRotate is a placeholder for setting display rotation via the
// GNOME Shell extension's DBus interface.
//
//nolint:unused // scaffold — will be called from display.go once implemented
func gnomeExtRotate(direction string) error {
	_ = direction
	// TODO: Map direction → transform enum and call ApplyMonitorsConfig
	return nil
}

// gnomeExtListOutputs is a placeholder for listing outputs via the GNOME
// Shell extension's DBus interface.
//
//nolint:unused // scaffold — will be called from display.go once implemented
func gnomeExtListOutputs() ([]Output, error) {
	// TODO: Call org.gnome.Mutter.DisplayConfig.GetResources, parse outputs
	return nil, nil
}
