#!/usr/bin/env python3
import sys
import os
import subprocess
import json
import time
import threading
import asyncio
import websockets
from http.server import SimpleHTTPRequestHandler
import socketserver
from gi.repository import GLib, Gio

BUS_NAME = "org.remote_studio.Daemon"
OBJECT_PATH = "/org/remote_studio/Daemon"

XML = """
<node>
  <interface name="org.remote_studio.Daemon">
    <property name="Status" type="s" access="read"/>
    <method name="Refresh"/>
    <signal name="StatusChanged">
      <arg name="status" type="s"/>
    </signal>
  </interface>
</node>
"""

class RemoteStudioDaemon:
    def __init__(self):
        self.status = "Idle"
        self.prev_users = 0
        self.active_ips = []
        self.poll_interval = 5

        # D-Bus Setup
        self.dbus_conn = Gio.bus_get_sync(Gio.BusType.SESSION, None)
        Gio.bus_own_name_on_connection(self.dbus_conn, BUS_NAME, Gio.BusNameOwnerFlags.NONE, None, None)
        
        self.node_info = Gio.DBusNodeInfo.new_for_xml(XML).interfaces[0]
        self.dbus_conn.register_object(OBJECT_PATH, self.node_info, self.handle_method, self.get_property, self.set_property)
        
        # Start Poll Loop
        GLib.timeout_add_seconds(self.poll_interval, self.poll_network)
        # Run immediately once
        self.poll_network()

    def get_property(self, connection, sender, object_path, interface_name, property_name, user_data):
        if property_name == "Status":
            try:
                json_status = subprocess.check_output("res status --json", shell=True, text=True).strip()
                data = json.loads(json_status)
                data["active_ips"] = getattr(self, "active_ips", [])
                dbus_str = json.dumps(data)
            except Exception:
                dbus_str = '{"mode": "Error"}'
            return GLib.Variant("s", dbus_str)

    def set_property(self, connection, sender, object_path, interface_name, property_name, value, user_data):
        return False

    def handle_method(self, connection, sender, object_path, interface_name, method_name, parameters, invocation, user_data):
        if method_name == "Refresh":
            self.poll_network()
            invocation.return_value(None)

    def emit_status_changed(self):
        broadcast_status_to_websockets()
        try:
            json_status = subprocess.check_output("res status --json", shell=True, text=True).strip()
            data = json.loads(json_status)
            data["active_ips"] = self.active_ips
            dbus_str = json.dumps(data)
        except Exception:
            dbus_str = '{"mode": "Error"}'
        self.dbus_conn.emit_signal(None, OBJECT_PATH, BUS_NAME, "StatusChanged", GLib.Variant("(s)", (dbus_str,)))

    def poll_network(self):
        try:
            ss_out = subprocess.check_output("ss -tnp 2>/dev/null | awk '/ESTAB/ && /rustdesk/{print $5}' | cut -d: -f1 | sort -u", shell=True, text=True)
            ips = [ip for ip in ss_out.strip().split('\n') if ip]
        except Exception:
            ips = []

        self.active_ips = ips
        users = len(ips)
        
        if users > 0 and self.prev_users == 0:
            trusted, peer_os = evaluate_peer_trust(ips)

            if trusted:
                print(f"Session connected from trusted IP. Detected OS: {peer_os}")
                self.status = "Active"
                self.emit_status_changed()
                
                # Auto profile logic
                profile = "mac" # default to mac
                if peer_os == "iOS":
                    profile = "ipad"
                elif peer_os == "macOS":
                    profile = "mac"
                elif peer_os == "windows":
                    profile = "fallback"
                elif peer_os == "linux":
                    profile = "fallback"
                    
                # Note: AUTO_SESSION check could go here if we want to respect the env var
                auto = os.environ.get("AUTO_SESSION", "true")
                if auto == "true":
                    subprocess.Popen(["res", "session", "start", profile])
            else:
                print(f"Session connected from UNTRUSTED IP. Ignored.")
                
        elif users == 0 and self.prev_users > 0:
            print("Session disconnected.")
            self.status = "Idle"
            self.emit_status_changed()
            auto = os.environ.get("AUTO_SESSION", "true")
            if auto == "true":
                subprocess.Popen(["res", "session", "stop"])

        self.prev_users = users
        
        # Trigger standard status file update for legacy/CLI apps
        try:
            # Write to STATUS_FILE directly or call `show_status` via res.sh
            # res.sh requires res command in PATH, or we call it absolutely
            res_bin = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "res.sh")
            subprocess.Popen(["bash", res_bin, "status"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        except Exception:
            pass
            
        broadcast_status_to_websockets()
        return True # Continue GLib loop

WEB_ROOT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "web")
WEB_DIST_DIR = os.path.join(WEB_ROOT, "dist")
WEB_DIR = WEB_DIST_DIR if os.path.isdir(WEB_DIST_DIR) else WEB_ROOT


def evaluate_peer_trust(ips):
    """Decide whether a connecting peer should be trusted, based on
    RES_LAN_MODE and the tailscale peer table.

    Returns (trusted: bool, peer_os: Optional[str]).

    Trust semantics (mirrors lib/core.sh::lan_mode_active + bash
    trust precedence):

      | LAN mode | tailscale | peer IP             | trusted |
      |----------|-----------|---------------------|---------|
      | off      | missing   | any                 | False   | (conservative)
      | off      | present   | 127.0.0.1           | True    |
      | off      | present   | in tailscale peer   | True    |
      | off      | present   | not in tailnet      | False   |
      | on       | missing   | any                 | True    | (LAN install)
      | on       | present   | any                 | True    | (LAN bypass)

    The `peer_os` field is only populated in the "tailscale present,
    peer in tailnet" case — for all LAN-mode paths we don't bother
    extracting it because every IP is trusted anyway.
    """
    trusted = True
    peer_os = None
    lan_mode = os.environ.get("RES_LAN_MODE", "").lower() in ("1", "true", "yes", "on")
    try:
        ts_status = subprocess.check_output(
            "tailscale status --json 2>/dev/null", shell=True, text=True
        )
        ts_data = json.loads(ts_status)
    except Exception:
        # tailscale not installed, or failed to run.
        if lan_mode:
            # LAN-only install: no tailnet, but the user opted in to
            # LAN, so every peer is implicitly trusted.
            return True, None
        # Conservative: tailscale-missing + tailnet mode is genuinely
        # suspicious. Treat as untrusted.
        return False, None

    if lan_mode:
        # LAN mode + tailscale installed: trust every peer (LAN bypass),
        # but still try to enrich peer_os from the tailscale table.
        for ip in ips:
            for peer_info in ts_data.get("Peer", {}).values():
                if ip in peer_info.get("TailscaleIPs", []):
                    peer_os = peer_info.get("OS", "unknown")
                    break
        return True, peer_os

    # tailscale present, LAN mode off: standard tailnet trust check.
    trusted = False
    for ip in ips:
        if ip == "127.0.0.1":
            trusted = True
            break
        for peer_info in ts_data.get("Peer", {}).values():
            if ip in peer_info.get("TailscaleIPs", []):
                trusted = True
                peer_os = peer_info.get("OS", "unknown")
                break
        if trusted:
            break
    return trusted, peer_os


def run_http_server():
    os.chdir(WEB_DIR)
    handler = SimpleHTTPRequestHandler
    with socketserver.TCPServer(("", 9999), handler) as httpd:
        print("Serving Web UI on http://0.0.0.0:9999")
        httpd.serve_forever()

connected_clients = set()

def broadcast_status_to_websockets():
    if not connected_clients or ws_loop is None:
        return
    try:
        json_status = subprocess.check_output("res status --json", shell=True, text=True).strip()
    except Exception:
        json_status = '{"mode": "Error", "status": "Error fetching status"}'
    
    async def _broadcast():
        message = json.dumps({"type": "status_full", "data": json.loads(json_status)})
        coros = [client.send(message) for client in connected_clients]
        if coros:
            await asyncio.gather(*coros, return_exceptions=True)
            
    asyncio.run_coroutine_threadsafe(_broadcast(), ws_loop)


async def ws_handler(websocket, path):
    connected_clients.add(websocket)
    try:
        try:
            json_status = subprocess.check_output("res status --json", shell=True, text=True).strip()
            await websocket.send(json.dumps({"type": "status_full", "data": json.loads(json_status)}))
        except Exception:
            pass
        async for message in websocket:
            data = json.loads(message)
            if data.get("action") == "command":
                subprocess.Popen(["res", data.get("cmd")])
            elif data.get("action") == "scale":
                subprocess.Popen(["gsettings", "set", "org.cinnamon.desktop.interface", "text-scaling-factor", str(data.get("val"))])
    finally:
        connected_clients.remove(websocket)

ws_loop = None

def run_ws_server():
    global ws_loop
    ws_loop = asyncio.new_event_loop()
    loop = ws_loop
    asyncio.set_event_loop(loop)
    start_server = websockets.serve(ws_handler, "0.0.0.0", 9998)
    loop.run_until_complete(start_server)
    loop.run_forever()

if __name__ == "__main__":
    daemon = RemoteStudioDaemon()
    print("Remote Studio Python Daemon running on D-Bus...")
    
    # Start web servers
    threading.Thread(target=run_http_server, daemon=True).start()
    threading.Thread(target=run_ws_server, daemon=True).start()
    
    loop = GLib.MainLoop()
    try:
        loop.run()
    except KeyboardInterrupt:
        print("Exiting...")
        sys.exit(0)
