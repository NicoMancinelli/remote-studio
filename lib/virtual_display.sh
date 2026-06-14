#!/usr/bin/env bash

# Remote Studio - Virtual Display Manager
# This experimental script uses xserver-xorg-video-dummy to spawn a headless display
# that exactly matches the incoming Tailscale peer's resolution.

DUMMY_CONF="/tmp/xorg-dummy.conf"

generate_dummy_conf() {
    local width="$1"
    local height="$2"
    
    cat > "$DUMMY_CONF" <<EOF
Section "Device"
    Identifier  "Configured Video Device"
    Driver      "dummy"
    VideoRam    256000
EndSection

Section "Monitor"
    Identifier  "Configured Monitor"
    HorizSync   15.0-100.0
    VertRefresh 15.0-200.0
    Modeline "CustomMode" $(cvt "$width" "$height" 60 | grep Modeline | cut -d' ' -f3-)
EndSection

Section "Screen"
    Identifier  "Default Screen"
    Monitor     "Configured Monitor"
    Device      "Configured Video Device"
    DefaultDepth 24
    SubSection "Display"
        Depth   24
        Modes   "CustomMode"
    EndSubSection
EndSection
EOF
    echo "$DUMMY_CONF"
}

start_virtual_display() {
    local width="$1"
    local height="$2"
    local display_num="${3:-1}" # e.g., :1
    
    generate_dummy_conf "$width" "$height"
    
    echo "Starting virtual display on $display_num at ${width}x${height}..."
    Xorg "$display_num" -config "$DUMMY_CONF" -nolisten tcp -noreset +extension GLX +extension RANDR +extension RENDER &
    local xorg_pid=$!
    
    echo "Virtual display started with PID $xorg_pid"
    echo "$xorg_pid" > "/tmp/remote-studio-virtual-display.pid"
}

stop_virtual_display() {
    if [ -f "/tmp/remote-studio-virtual-display.pid" ]; then
        local pid
        pid=$(cat "/tmp/remote-studio-virtual-display.pid")
        kill "$pid" 2>/dev/null || true
        rm -f "/tmp/remote-studio-virtual-display.pid"
        echo "Virtual display stopped."
    fi
}
