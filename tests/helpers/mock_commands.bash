# Mock external commands for unit tests.
# Source this file in bats setup() when you need to stub display/network tools.

xrandr() { echo "HDMI-1 connected primary 2560x1664+0+0"; }
gsettings() {
    case "$1 $2" in
        "get org.cinnamon") echo "true" ;;
        "get org.cinnamon.desktop.screensaver") echo "true" ;;
        *) return 0 ;;
    esac
}
glxinfo() { echo "OpenGL renderer string: GeForce GTX 1080"; }
tailscale() {
    case "$1" in
        ip)     echo "100.1.2.3" ;;
        status) echo "Tailscale is running." ;;
        *)      return 0 ;;
    esac
}
systemctl() {
    case "$2" in
        rustdesk)   echo "active" ;;
        tailscaled) echo "active" ;;
        *)          echo "inactive" ;;
    esac
}
xgamma() { echo "Red  1.000, Green  1.000, Blue  1.000"; }
cvt()    { echo "# 2560x1664 59.95 Hz (CVT)"; echo "Modeline \"2560x1664_60.00\"  348.50  2560 2752 3032 3504  1664 1667 1677 1724 -hsync +vsync"; }

export -f xrandr gsettings glxinfo tailscale systemctl xgamma cvt
