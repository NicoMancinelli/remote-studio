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
        GLib.timeout_add_seconds(self.poll_interval * 1000, self.poll_network)
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
            # Detect OS
            trusted = True
            peer_os = None
            try:
                ts_status = subprocess.check_output("tailscale status --json 2>/dev/null", shell=True, text=True)
                ts_data = json.loads(ts_status)
                trusted = False
                for ip in ips:
                    if ip == "127.0.0.1":
                        trusted = True
                        break
                    for peer_key, peer_info in ts_data.get("Peer", {}).items():
                        if ip in peer_info.get("TailscaleIPs", []):
                            trusted = True
                            peer_os = peer_info.get("OS", "unknown")
                            break
            except Exception:
                pass # tailscale might not be installed

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

WEB_DIR = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "web")

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
