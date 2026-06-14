import re

with open("lib/tui.sh", "r") as f:
    lines = f.readlines()

new_lines = []

def process_line(line):
    # We will just replace common whiptail sizes
    # e.g., 22 84 10 -> "$T_LINES" "$T_COLS" "$T_MENU"
    # and 10 70 -> "$T_LINES" "$T_COLS"
    # But only inside whiptail commands. Since they often span multiple lines, 
    # it's better to do a global string replace of specific patterns 
    # or just use a regex on the whole file.
    pass

with open("lib/tui.sh", "r") as f:
    content = f.read()

# Add tui_get_dims at the top
header_func = """tui_get_dims() {
    T_LINES=$(tput lines 2>/dev/null || echo 24)
    T_COLS=$(tput cols 2>/dev/null || echo 90)
    T_LINES=$(( T_LINES > 6 ? T_LINES - 4 : 20 ))
    T_COLS=$(( T_COLS > 10 ? T_COLS - 8 : 82 ))
    T_MENU=$(( T_LINES > 10 ? T_LINES - 10 : 10 ))
}
"""

content = content.replace("tui_header() {", header_func + "\ntui_header() {")

# Fix tui_header wdata parsing and empty vars
old_tui_header = """    wdata=$(get_warning_summary_cached); wcount=${wdata%%|*}; wmsg=${wdata#*|}
    renderer=$(get_renderer_summary 2>/dev/null | sed 's/.*NVIDIA.*/NVIDIA/;s/.*AMD.*/AMD/;s/.*Intel.*/Intel/;s/.*llvmpipe.*/SW-render/')
    rustdesk_st=$(systemctl is-active rustdesk 2>/dev/null || echo "?")
    session_st="$([ -f "$SESSION_FILE" ] && echo "active" || echo "idle")"
    printf 'Mode: %s (%s)  |  IP: %s  |  Session: %s\\nRustDesk: %s  |  Renderer: %s  |  Warnings: %s' \\
        "$mode" "$res_str" "${ip:-none}" "$session_st" \\
        "$rustdesk_st" "$renderer" \\
        "$wcount$([ "$wcount" -gt 0 ] && printf ' (%s)' "$wmsg" || true)" """

new_tui_header = """    wdata=$(get_warning_summary_cached || echo "0|OK")
    wcount=${wdata%%|*}
    wmsg=${wdata#*|}
    if ! [[ "$wcount" =~ ^[0-9]+$ ]]; then wcount=0; wmsg="Error"; fi
    renderer=$(get_renderer_summary 2>/dev/null | head -n 1 | sed 's/.*NVIDIA.*/NVIDIA/;s/.*AMD.*/AMD/;s/.*Intel.*/Intel/;s/.*llvmpipe.*/SW-render/')
    renderer=${renderer:-unknown}
    rustdesk_st=$(systemctl is-active rustdesk 2>/dev/null || echo "?")
    session_st="$([ -f "$SESSION_FILE" ] && echo "active" || echo "idle")"

    local wtext="$wcount"
    [ "$wcount" -gt 0 ] && wtext="$wcount ($wmsg)"

    printf 'Mode: %s (%s)  |  IP: %s  |  Session: %s\\nRustDesk: %s  |  Renderer: %s  |  Warnings: %s' \\
        "${mode:-unknown}" "${res_str:-unknown}" "${ip:-none}" "$session_st" \\
        "$rustdesk_st" "$renderer" "$wtext" """

content = content.replace(old_tui_header, new_tui_header)

# Before we replace sizes, let's inject a call to tui_get_dims at the top of each tui_* function
# Except tui_get_dims and tui_header
for func in re.findall(r'^(tui_[a-z_]+)\(\) \{', content, re.M):
    if func not in ('tui_get_dims', 'tui_header'):
        content = re.sub(r'^' + func + r'\(\) \{\n', f"{func}() {{\n    tui_get_dims\n", content, flags=re.M)

# Also in confirm_action
content = re.sub(r'confirm_action\(\) \{ whiptail --title "Confirm" --yesno "\$1" 10 70; \}',
                 'confirm_action() { tui_get_dims; whiptail --title "Confirm" --yesno "$1" "$T_LINES" "$T_COLS"; }', content)

# Now replace hardcoded whiptail sizes
# Pattern: \d+ \d+ \d+  (lines cols menu_height)
# Pattern: \d+ \d+      (lines cols)
# We can find all whiptail calls and their sizes.
# But simply doing regex replacements for common combinations:
content = re.sub(r' 22 84 10 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 18 70 6 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 24 90 18 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 22 70 12 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 12 55 3 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 24 86 10 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 20 70 10 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 12 52 4 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 20 72 8 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 24 88 10 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 22 88 10 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 24 88 12 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 14 68 5 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' 15 50 5 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)
content = re.sub(r' "$lines" "$cols" 6 ', ' "$T_LINES" "$T_COLS" "$T_MENU" ', content)

# 2 arguments:
content = re.sub(r' 7 62', ' "$T_LINES" "$T_COLS"', content)
content = re.sub(r' 7 64', ' "$T_LINES" "$T_COLS"', content)
content = re.sub(r' 7 60', ' "$T_LINES" "$T_COLS"', content)
content = re.sub(r' 7 55', ' "$T_LINES" "$T_COLS"', content)
content = re.sub(r' 7 65', ' "$T_LINES" "$T_COLS"', content)
content = re.sub(r' 9 60 ', ' "$T_LINES" "$T_COLS" ', content)
content = re.sub(r' 8 60;', ' "$T_LINES" "$T_COLS";', content)
content = re.sub(r' 7 50', ' "$T_LINES" "$T_COLS"', content)
content = re.sub(r' 8 50;', ' "$T_LINES" "$T_COLS";', content)
content = re.sub(r' 9 50 ', ' "$T_LINES" "$T_COLS" ', content)
content = re.sub(r' 9 58 ', ' "$T_LINES" "$T_COLS" ', content)
content = re.sub(r' 8 64', ' "$T_LINES" "$T_COLS"', content)
content = re.sub(r' 9 55 ', ' "$T_LINES" "$T_COLS" ', content)

# In tui_dashboard, there is custom logic:
dashboard_old = """    local lines cols body mode res_str renderer rustdesk_st tailscale_st session_info recent_log
    while true; do
        lines=$(tput lines 2>/dev/null || echo 24)
        cols=$(tput cols 2>/dev/null || echo 90)
        lines=$(( lines > 6 ? lines - 2 : 22 ))
        cols=$(( cols > 10 ? cols - 4 : 86 ))"""

dashboard_new = """    local body mode res_str renderer rustdesk_st tailscale_st session_info recent_log
    while true; do
        tui_get_dims"""

content = content.replace(dashboard_old, dashboard_new)

# In tui_config, there is custom logic:
config_old = """    local choice key val lines cols
    lines=$(tput lines 2>/dev/null || echo 24)
    cols=$(tput cols 2>/dev/null || echo 90)
    lines=$(( lines > 6 ? lines - 2 : 22 ))
    cols=$(( cols > 10 ? cols - 4 : 86 ))
    while true; do"""

config_new = """    local choice key val
    while true; do
        tui_get_dims"""
content = content.replace(config_old, config_new)

# In run_panel_command:
rpc_old = """    local tmp lines cols
    tmp=$(mktemp)
    lines=$(tput lines 2>/dev/null || echo 24)
    cols=$(tput cols 2>/dev/null || echo 90)
    lines=$(( lines > 6 ? lines - 2 : 22 ))
    cols=$(( cols > 10 ? cols - 4 : 86 ))
    { echo "$ $*"; echo; "$@"; } > "$tmp" 2>&1
    whiptail --title "$title" --scrolltext --textbox "$tmp" "$lines" "$cols" """

rpc_new = """    local tmp
    tmp=$(mktemp)
    tui_get_dims
    { echo "$ $*"; echo; "$@"; } > "$tmp" 2>&1
    whiptail --title "$title" --scrolltext --textbox "$tmp" "$T_LINES" "$T_COLS" """

content = content.replace(rpc_old, rpc_new)

# In tui_log_viewer:
log_old = """                if [ ! -s "$tmp" ]; then
                    whiptail --msgbox "No matches for '$filter'." "$T_LINES" "$T_COLS"
                else
                    tlines=$(tput lines 2>/dev/null || echo 24); tcols=$(tput cols 2>/dev/null || echo 90)
                    tlines=$(( tlines > 6 ? tlines - 2 : 22 )); tcols=$(( tcols > 10 ? tcols - 4 : 86 ))
                    whiptail --title "Log filter: $filter" --scrolltext --textbox "$tmp" "$tlines" "$tcols"
                fi"""

log_new = """                if [ ! -s "$tmp" ]; then
                    whiptail --msgbox "No matches for '$filter'." "$T_LINES" "$T_COLS"
                else
                    tui_get_dims
                    whiptail --title "Log filter: $filter" --scrolltext --textbox "$tmp" "$T_LINES" "$T_COLS"
                fi"""

content = content.replace(log_old, log_new)

# One more check for dashboard whiptail invocation
content = content.replace('"$lines" "$cols"', '"$T_LINES" "$T_COLS"')


with open("lib/tui.sh", "w") as f:
    f.write(content)

